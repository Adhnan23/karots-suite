package productplus

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/productplus/migrations"
)

func TestAttributesScopeAndRules(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_productplus"); err != nil {
		t.Fatal(err)
	}
	s, ctx := NewStore(db), context.Background()
	tag := time.Now().Format("150405.000000")
	var parent, child, pid int64
	if err := db.Get(&parent, `INSERT INTO categories (name) VALUES ($1) RETURNING id`, "PP parent "+tag); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&child, `INSERT INTO categories (name, parent_id) VALUES ($1, $2) RETURNING id`, "PP child "+tag, parent); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&pid, `INSERT INTO products (name, category_id, unit_id) VALUES ($1, $2, (SELECT id FROM units LIMIT 1)) RETURNING id`, "PP item "+tag, child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM products WHERE id = $1`, pid)                                      //nolint:errcheck
		db.Exec(`DELETE FROM pp_fields WHERE label LIKE $1`, "%"+tag)                           //nolint:errcheck
		db.Exec(`DELETE FROM categories WHERE id = ANY(ARRAY[$1,$2]::bigint[])`, child, parent) //nolint:errcheck
	})
	id, err := s.CreateField(ctx, Field{Label: "Form " + tag, Type: "select", Options: []string{"Tablet", "Capsule"}, IsActive: true}, []int64{parent})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Field(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	set := s.attributes()
	attrs, err := set.Attributes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(attrs, func(a plugin.ProductAttribute) bool { return a.Key == f.Key })
	if i < 0 || !slices.Contains(attrs[i].CategoryIDs, child) || !slices.Contains(attrs[i].CategoryIDs, parent) || attrs[i].Type != "select" {
		t.Fatalf("scope reaches sub-categories: %+v", attrs)
	}
	if err := set.Set(ctx, pid, f.Key, "Liquid"); err == nil {
		t.Fatal("a value off the options was saved")
	}
	if err := set.Set(ctx, pid, f.Key, "tablet"); err != nil {
		t.Fatal(err)
	}
	vals, err := set.Values(ctx, []int64{pid})
	if err != nil || vals[pid][f.Key] != "Tablet" {
		t.Fatalf("the field's own spelling: %+v %v", vals, err)
	}
	known, err := set.Known(ctx, f.Key, 5)
	if err != nil || len(known) != 1 || known[0].Value != "Tablet" {
		t.Fatalf("%+v %v", known, err)
	}
	if err := set.Set(ctx, pid, f.Key, ""); err != nil {
		t.Fatal(err)
	}
	vals, _ = set.Values(ctx, []int64{pid})
	if vals[pid][f.Key] != "" {
		t.Fatalf("undo back to blank: %+v", vals)
	}
}
