package requests

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
	"karots-suite/plugins/requests/migrations"
)

// A karots-pos shop's requests restore as they are — no mapping: each row is one
// ask, and a converted one reads as to call / done from its product's stock and
// whether it left a phone.
func TestKarotsPosRequestsRestore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "requests_restore_db"
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
	noErr(t, appdb.RunMigrations(db.DB, coremigrations.FS))
	noErr(t, appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_requests"))
	noErr(t, db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Old shop') RETURNING id`))
	var stocked, empty int64
	for _, id := range []*int64{&stocked, &empty} {
		noErr(t, db.Get(id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('Old shop item', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 200) RETURNING id`))
	}
	setStock(t, db, stocked, "4")

	ts := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"requests"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001 only).
		{Name: "requests", Columns: []string{"id", "note", "customer_name", "customer_phone", "status", "product_id", "created_by", "created_at", "updated_at"},
			Rows: [][]*string{
				{s("1"), s("Red umbrella"), s("Nimal"), s("0771234567"), s("open"), nil, nil, &ts, &ts},
				{s("2"), s("Torch"), s("Sunil"), s("0779998887"), s("converted"), id(stocked), nil, &ts, &ts},
				{s("3"), s("Glue"), nil, nil, s("converted"), id(empty), nil, &ts, &ts},
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
	st := NewStore(db)
	want := map[string]string{"open": "Red umbrella", "call": "Torch", "done": "Glue"}
	for tab, note := range want {
		rows, _, err := st.List(context.Background(), tab, "", 50, 0)
		noErr(t, err)
		if len(rows) != 1 || rows[0].Note != note || len(rows[0].Asks) != 1 {
			t.Fatalf("tab %s: want %q with one ask, got %+v", tab, note, rows)
		}
	}
	// A new request after the restore gets a fresh id (the sequence follows on).
	if n, err := st.Add(context.Background(), "After restore", "", "", 0); err != nil || n <= 3 {
		t.Fatalf("new id %d: %v", n, err)
	}
}
