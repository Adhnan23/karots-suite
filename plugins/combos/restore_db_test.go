package combos

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"karots-suite/internal/backup"
	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/combos/migrations"
)

// A karots-pos shop's combos restore as they are — no mapping: no dates (always
// on), percent / fixed mean the same, no sales history yet.
func TestKarotsPosCombosRestore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "combos_restore_db"
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(appdb.RunMigrations(db.DB, coremigrations.FS))
	must(appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_combos"))
	must(db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Old shop') RETURNING id`))
	var milk, bun int64
	for _, id := range []*int64{&milk, &bun} {
		must(db.Get(id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('Old shop item', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 100) RETURNING id`))
	}

	ts := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"combos"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001 only).
		{Name: "combos", Columns: []string{"id", "name", "discount_type", "discount_value", "is_active", "created_at"},
			Rows: [][]*string{
				{s("1"), s("Breakfast"), s("percent"), s("10"), s("true"), &ts},
				{s("2"), s("Buns 3+1"), s("fixed"), s("100"), s("true"), &ts},
			}},
		{Name: "combo_items", Columns: []string{"id", "combo_id", "product_id", "qty"},
			Rows: [][]*string{
				{s("1"), s("1"), id(milk), s("1")},
				{s("2"), s("1"), id(bun), s("1")},
				{s("3"), s("2"), id(bun), s("4")},
			}},
	}}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	json.NewEncoder(gz).Encode(arc) //nolint:errcheck
	gz.Close()
	rep, err := backup.Restore(context.Background(), db, &buf)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(rep.DroppedColumns) > 0 || len(rep.SkippedTables) > 0 {
		t.Fatalf("restore left things out: %+v", rep)
	}
	till, err := NewStore(db).Till(context.Background())
	must(err)
	if len(till) != 2 || till[0].StartsOn != nil || till[0].EndsOn != nil || till[0].Window != "on" {
		t.Fatalf("both old combos must be on the till, undated: %+v", till)
	}
	for _, c := range till {
		if c.Name == "Buns 3+1" && (len(c.Items) != 1 || !c.FullPrice.Equal(d("400"))) {
			t.Fatalf("multi-buy members: %+v", c)
		}
	}
	// A new combo after the restore gets a fresh id (the sequence follows on).
	n, err := NewStore(db).Create(context.Background(), Input{Name: "After", DiscountType: "percent", DiscountValue: d("5"), Items: []ItemInput{{milk, d("2")}}})
	if err != nil || n <= 2 {
		t.Fatalf("new id %d: %v", n, err)
	}
}
