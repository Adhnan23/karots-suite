package aicatalog

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"testing"

	"karots-suite/internal/backup"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/aicatalog/migrations"

	"github.com/jmoiron/sqlx"
)

// A karots-pos shop at 0007 restores: learnt items, an optimize run and its
// settings (own manual_mode) come across; the staging table and default_markup
// are left out; the API key comes in blank and never goes out in a dump.
func TestKarotsPosAICatalogRestores(t *testing.T) {
	db := freshDB(t, "aicatalog_restore_db")
	ctx := context.Background()
	(&plugin.Registry{}).AddDataGroup(dataGroup)
	st := NewStore(db)
	if s, err := st.GetSettings(ctx); err != nil || !s.ManualMode {
		t.Fatalf("a fresh install should start in paste mode: %+v %v", s, err)
	}
	var pid int64
	mustExec(t, db, &pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('6201 ZZ', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 100, 200) RETURNING id`)
	str := func(v string) *string { return &v }
	id := str(strconv.FormatInt(pid, 10))
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"aicatalog"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001–0007).
		{Name: "aicatalog_settings", Columns: []string{"id", "provider", "base_url", "model", "api_key", "default_markup", "manual_mode", "custom_prompt", "web_search", "naming_style"},
			Rows: [][]*string{{str("1"), str("groq"), str("https://api.groq.com/openai/v1"), str("llama"), str("sk-secret"), str("1.400"), str("false"), str("be brief"), str("true"), str("Brand Size")}}},
		{Name: "aicatalog_items", Columns: []string{"product_id", "resolved_name", "suggested_category", "specs", "user_explanation", "source", "raw_query", "created_at", "updated_at"},
			Rows: [][]*string{{id, str("6201-2Z Ball Bearing"), str("Bearings"), str("12x32x10 mm"), str(""), str("ai_confident"), str("6201 ZZ"), str("2026-01-01T00:00:00Z"), str("2026-01-01T00:00:00Z")}}},
		{Name: "aicatalog_optimize_runs", Columns: []string{"id", "created_at", "applied_by", "summary", "undo", "reverted_at"},
			Rows: [][]*string{{str("3"), str("2026-01-02T00:00:00Z"), nil, str("1 rename"), str(`[{"kind":"rename","id":1,"name":"old"}]`), nil}}},
		{Name: "aicatalog_mock_products", Columns: []string{"id", "name", "status", "created_product_id"},
			Rows: [][]*string{{str("1"), str("draft one"), str("draft"), nil}, {str("2"), str("done"), str("committed"), id}}},
	}}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	json.NewEncoder(gz).Encode(arc) //nolint:errcheck
	gz.Close()
	rep, err := backup.Restore(ctx, db, &buf)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(rep.SkippedTables) != 1 || rep.SkippedTables[0] != "aicatalog_mock_products" {
		t.Errorf("skipped %v, want the staging table", rep.SkippedTables)
	}
	s, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s.ManualMode || s.Provider != "groq" || s.NamingStyle != "Brand Size" || s.APIKey != "" {
		t.Errorf("settings after restore: %+v (want own mode, key blank)", s)
	}
	if it, _ := st.GetItem(ctx, pid); it == nil || it.ResolvedName != "6201-2Z Ball Bearing" {
		t.Errorf("learnt item lost: %+v", it)
	}
	if n := count(t, db, `SELECT count(*) FROM aicatalog_optimize_runs WHERE id = 3`); n != 1 {
		t.Error("optimize run lost")
	}
	// The key typed after a restore never leaves in a dump.
	mustExec(t, db, nil, `UPDATE aicatalog_settings SET api_key = 'sk-new'`)
	buf.Reset()
	if err := backup.Dump(ctx, db, "now", &buf, backup.DumpOpts{}); err != nil {
		t.Fatal(err)
	}
	out, err := backup.ReadArchive(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range out.Tables {
		if tb.Name != "aicatalog_settings" {
			continue
		}
		for i, c := range tb.Columns {
			if c == "api_key" && (tb.Rows[0][i] == nil || *tb.Rows[0][i] != "") {
				t.Errorf("api_key in the dump: %v", tb.Rows[0][i])
			}
		}
	}
}

// freshDB creates a throwaway database with core + this plugin migrated.
func freshDB(t *testing.T, name string) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) }) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrations(db.DB, coremigrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_aicatalog"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, nil, `INSERT INTO categories (name) VALUES ('Old shop')`)
	return db
}

func mustExec(t *testing.T, db *sqlx.DB, dest *int64, q string, args ...any) {
	t.Helper()
	var err error
	if dest != nil {
		err = db.Get(dest, q, args...)
	} else {
		_, err = db.Exec(q, args...)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, db *sqlx.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Get(&n, q, args...); err != nil {
		t.Fatal(err)
	}
	return n
}
