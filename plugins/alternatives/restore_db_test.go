package alternatives

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
	"karots-suite/plugins/alternatives/migrations"
)

// A karots-pos shop's groups restore as they are — no mapping: named tiers stay
// tiers, reorder levels keep judging stock, search and pins work.
func TestKarotsPosAlternativesRestore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "alternatives_restore_db"
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
	must(appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_alternatives"))
	must(db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Old shop') RETURNING id`))
	var king, sand int64
	for _, id := range []*int64{&king, &sand} {
		must(db.Get(id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('Old shop item', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 100) RETURNING id`))
	}

	ts := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	db.MustExec(`UPDATE products SET name = 'Old shop Kingston' WHERE id = $1`, king)
	db.MustExec(`UPDATE stock SET quantity = CASE WHEN product_id = $1 THEN 0 ELSE 14 END WHERE product_id IN ($1, $2)`, king, sand)
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"alternatives"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001).
		{Name: "alt_groups", Columns: []string{"id", "name", "sort_order", "is_active", "created_at"},
			Rows: [][]*string{{s("1"), s("USB 32GB"), s("0"), s("true"), &ts}}},
		{Name: "alt_tiers", Columns: []string{"id", "group_id", "name", "reorder_level", "sort_order", "is_active"},
			Rows: [][]*string{{s("1"), s("1"), s("Genuine"), s("10"), s("0"), s("true")}, {s("2"), s("1"), s("Cheap"), s("0"), s("0"), s("true")}}},
		{Name: "alt_members", Columns: []string{"product_id", "tier_id", "created_at"},
			Rows: [][]*string{{id(king), s("1"), &ts}, {id(sand), s("1"), &ts}}},
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
	ctx := context.Background()
	g, err := st.Group(ctx, 1)
	must(err)
	if len(g.Tiers) != 2 || g.Tiers[0].Name != "Genuine" || g.Tiers[0].Total.String() != "14" || g.Tiers[0].Low || g.Products != 2 {
		t.Fatalf("old group: %+v", g)
	}
	cov, err := st.CoverageFor(ctx, []int64{king})
	must(err)
	if !cov[king].Covered {
		t.Fatalf("Genuine (14 > 10) still covers Kingston: %+v", cov)
	}
	alts, err := st.Alternatives(ctx, king, 3)
	must(err)
	if len(alts) != 1 || alts[0].ProductID != sand {
		t.Fatalf("swap offers SanDisk: %+v", alts)
	}
	// Old tiers all share sort_order 0: moving still works, and new rows get fresh ids.
	must(st.MoveTier(ctx, 2, true))
	if g, _ = st.Group(ctx, 1); g.Tiers[0].Name != "Cheap" {
		t.Fatalf("move on old rows: %+v", g.Tiers)
	}
	n, err := st.CreateGroup(ctx, "After")
	if err != nil || n <= 1 {
		t.Fatalf("new id %d: %v", n, err)
	}
}
