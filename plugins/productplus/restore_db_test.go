package productplus

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strconv"
	"testing"

	"karots-suite/internal/backup"
	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/productplus/migrations"
)

// A karots-pos shop's custom fields restore as they are — no mapping: with no
// pp_field_categories rows every field applies to every product, as before, and
// a required field with a default is never missing.
func TestKarotsPosProductPlusRestore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "productplus_restore_db"
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
	must(appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_productplus"))
	var cat1, cat2, p1, p2 int64
	must(db.Get(&cat1, `INSERT INTO categories (name) VALUES ('Phones') RETURNING id`))
	must(db.Get(&cat2, `INSERT INTO categories (name) VALUES ('Covers') RETURNING id`))
	for _, x := range []struct {
		id  *int64
		cat int64
	}{{&p1, cat1}, {&p2, cat2}} {
		must(db.Get(x.id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('Old shop item', $1, (SELECT id FROM units LIMIT 1), 0, 200) RETURNING id`, x.cat))
	}

	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	ts := s("2026-01-05T10:00:00Z")
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"productplus"}, Tables: []backup.Table{
		// karots-pos columns exactly (0001–0004).
		{Name: "pp_fields", Columns: []string{"id", "key", "label", "type", "default_value", "required", "searchable", "options",
			"sort_order", "is_active", "created_at", "hint", "show_at_till", "print_on_label", "show_in_intake"},
			Rows: [][]*string{
				{s("1"), s("model_no"), s("Model No"), s("text"), s("N/A"), s("true"), s("true"), nil, s("0"), s("true"), ts, s(""), s("true"), s("true"), s("false")},
				{s("2"), s("waterproof"), s("Waterproof"), s("bool"), s(""), s("false"), s("true"), nil, s("1"), s("true"), ts, s(""), s("true"), s("false"), s("true")},
				{s("3"), s("grade"), s("Grade"), s("select"), s("A"), s("false"), s("false"), s(`["A","B","C"]`), s("2"), s("true"), ts, s("Shop grade"), s("false"), s("false"), s("false")},
			}},
		{Name: "pp_values", Columns: []string{"field_id", "product_id", "value"},
			Rows: [][]*string{
				{s("1"), id(p1), s("SM-A055F")},
				{s("2"), id(p1), s("1")},
				{s("3"), id(p2), s("B")},
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

	ctx := context.Background()
	st := NewStore(db)
	var scoped int
	must(db.Get(&scoped, `SELECT count(*) FROM pp_field_categories`))
	if scoped != 0 {
		t.Fatalf("pp_field_categories: %d rows", scoped)
	}
	for _, x := range []struct{ pid, cat int64 }{{p1, cat1}, {p2, cat2}} {
		form, err := st.FormFields(ctx, x.pid, x.cat, "form")
		must(err)
		if len(form) != 3 {
			t.Fatalf("product %d sees %d fields, want all 3", x.pid, len(form))
		}
	}
	if v, _ := formValue(t, st, p2, cat2, Field{ID: 1}); v != "N/A" {
		t.Fatalf("p2 model no reads the default: %q", v)
	}
	stats, err := st.Stats(ctx)
	must(err)
	if stats[1].Missing != 0 {
		t.Fatalf("a required field with a default is missing %d", stats[1].Missing)
	}
	if ids, _ := st.MatchProductIDs(ctx, "a055"); !slices.Equal(ids, []int64{p1}) {
		t.Fatalf("search by value: %v", ids)
	}
	if ids, _ := st.MatchProductIDs(ctx, "waterproof"); !slices.Equal(ids, []int64{p1}) {
		t.Fatalf("search a yes/no by label: %v", ids)
	}
	// A field added after the restore gets a fresh id (the sequence follows on).
	if n, err := st.CreateField(ctx, Field{Label: "Colour", Type: "text"}, nil); err != nil || n <= 3 {
		t.Fatalf("new field id %d: %v", n, err)
	}
}
