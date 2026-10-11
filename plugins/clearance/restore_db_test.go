package clearance

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
	"karots-suite/plugins/clearance/migrations"
)

// A karots-pos shop's clearance tables restore as they are — no mapping: an old
// approval is live until fresh stock arrives, an old dismissal reads as snoozed
// from its own updated_at.
func TestKarotsPosClearanceRestores(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "clearance_restore_db"
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
	if err := appdb.RunMigrations(db.DB, coremigrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_clearance"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, nil, `INSERT INTO categories (name) VALUES ('Old shop')`)
	// Two products in stock, never sold; their lots arrived 30 days ago.
	var a, b int64
	for _, id := range []*int64{&a, &b} {
		mustExec(t, db, id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('Old shop item', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 100, 200) RETURNING id`)
		mustExec(t, db, nil, `INSERT INTO stock (product_id, quantity) VALUES ($1, 5) ON CONFLICT (product_id) DO UPDATE SET quantity = 5`, *id)
		mustExec(t, db, nil, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, created_at) VALUES ($1, 5, 5, 100, now() - interval '30 days')`, *id)
	}
	ts := func(d time.Duration) *string { v := time.Now().Add(-d).UTC().Format(time.RFC3339); return &v }
	str := func(v string) *string { return &v }
	ids := func(id int64) *string { v := strconv.FormatInt(id, 10); return &v }
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"clearance"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001 + 0002 + 0003).
		{Name: "clearance_markdowns", Columns: []string{"product_id", "discount_type", "discount_value", "status", "approved_by", "created_at", "updated_at"},
			Rows: [][]*string{
				{ids(a), str("percent"), str("20"), str("approved"), nil, ts(20 * 24 * time.Hour), ts(20 * 24 * time.Hour)},
				{ids(b), str("percent"), str("0"), str("dismissed"), nil, ts(10 * 24 * time.Hour), ts(10 * 24 * time.Hour)},
			}},
		{Name: "clearance_settings", Columns: []string{"id", "stale_days", "default_percent", "min_margin_percent", "expiry_days", "dead_months"},
			Rows: [][]*string{{str("1"), str("60"), str("20"), str("5"), str("14"), str("6")}}},
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
	if !isLive(t, st, a) {
		t.Error("an old approval with no fresher stock should be live")
	}
	if c := candidate(t, st, b); c == nil || c.Status != "snoozed" || c.SnoozedUntil == nil {
		t.Errorf("an old dismissal should read as snoozed, got %+v", c)
	} else if d := time.Until(*c.SnoozedUntil); d < 49*24*time.Hour || d > 51*24*time.Hour {
		t.Errorf("snoozed until %v: want ~50 days from now (60 − 10)", c.SnoozedUntil)
	}
}
