package productplus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/productplus/migrations"

	"github.com/jmoiron/sqlx"
)

// fixture is one test's own categories (parent > child, other) and products.
type fixture struct {
	s                    *Store
	db                   *sqlx.DB
	tag                  string
	parent, child, other int64
}

func setup(t *testing.T) *fixture {
	t.Helper()
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
	f := &fixture{s: NewStore(db), db: db, tag: fmt.Sprintf("zpp%d", time.Now().UnixNano())}
	cat := func(name string, parent *int64) int64 {
		var id int64
		if err := db.Get(&id, `INSERT INTO categories (name, parent_id) VALUES ($1, $2) RETURNING id`, f.tag+" "+name, parent); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.parent = cat("Parent", nil)
	f.child = cat("Child", &f.parent)
	f.other = cat("Other", nil)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM pp_fields WHERE label LIKE $1`, f.tag+"%")                           //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE name LIKE $1`, f.tag+"%")                             //nolint:errcheck
		db.Exec(`DELETE FROM categories WHERE name LIKE $1 AND parent_id IS NOT NULL`, f.tag+"%") //nolint:errcheck
		db.Exec(`DELETE FROM categories WHERE name LIKE $1`, f.tag+"%")                           //nolint:errcheck
	})
	return f
}

func (f *fixture) product(t *testing.T, name string, cat int64) int64 {
	t.Helper()
	var id int64
	if err := f.db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, $2, (SELECT id FROM units LIMIT 1), 0, 10) RETURNING id`, f.tag+" "+name, cat); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *fixture) field(t *testing.T, fl Field, scope ...int64) Field {
	t.Helper()
	fl.Label = f.tag + " " + fl.Label
	id, err := f.s.CreateField(context.Background(), fl, scope)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.s.Field(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return *got
}

func (f *fixture) set(t *testing.T, pid int64, fl Field, v string) {
	t.Helper()
	if _, errs, err := f.s.SetValues(context.Background(), []Cell{{ProductID: pid, FieldID: fl.ID, Value: v}}); err != nil || len(errs) > 0 {
		t.Fatalf("set %s=%q: %v %+v", fl.Label, v, err, errs)
	}
}

func (f *fixture) stored(pid int64, fl Field) (string, bool) {
	var v string
	err := f.db.Get(&v, `SELECT value FROM pp_values WHERE field_id=$1 AND product_id=$2`, fl.ID, pid)
	return v, err == nil
}

// mine keeps the rows of this test's fields (labels start with its tag): the
// shared dev DB may hold other fields, such as e2e or screenshot data.
func (f *fixture) mine(rows []plugin.DetailRow, _ error) []plugin.DetailRow {
	return slices.DeleteFunc(rows, func(r plugin.DetailRow) bool { return !strings.HasPrefix(r.Label, f.tag) })
}

// mineMeta is a list line cut down to this test's fields.
func (f *fixture) mineMeta(m string) string {
	parts := slices.DeleteFunc(strings.Split(m, " · "), func(p string) bool { return !strings.HasPrefix(p, f.tag) })
	return strings.Join(parts, " · ")
}

func formValue(t *testing.T, s *Store, pid, cat int64, fl Field) (string, bool) {
	t.Helper()
	rows, err := s.FormFields(context.Background(), pid, cat, "form")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Field.ID == fl.ID {
			return r.Value, true
		}
	}
	return "", false
}

func TestScope(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	inChild := f.product(t, "In child", f.child)
	outside := f.product(t, "Outside", f.other)
	model := f.field(t, Field{Label: "Model", Type: "text", Searchable: true, ShowAtTill: true}, f.parent)
	f.set(t, inChild, model, "XJ-900")

	if v, ok := formValue(t, f.s, inChild, f.child, model); !ok || v != "XJ-900" {
		t.Fatalf("child inherits parent scope: %q %v", v, ok)
	}
	if _, ok := formValue(t, f.s, outside, f.other, model); ok {
		t.Fatal("field shown outside its scope")
	}
	// Moved out: hidden everywhere, value kept; moved back: shows again.
	f.db.Exec(`UPDATE products SET category_id=$1 WHERE id=$2`, f.other, inChild) //nolint:errcheck
	if _, ok := formValue(t, f.s, inChild, f.other, model); ok {
		t.Fatal("form shows an out-of-scope field")
	}
	if rows := f.mine(f.s.TillRows(ctx, inChild)); len(rows) != 0 {
		t.Fatalf("till rows out of scope: %+v", rows)
	}
	if m, _ := f.s.MetaFor(ctx, []int64{inChild}); f.mineMeta(m[inChild]) != "" {
		t.Fatalf("meta out of scope: %q", m[inChild])
	}
	if ids, _ := f.s.MatchProductIDs(ctx, "xj-900"); slices.Contains(ids, inChild) {
		t.Fatal("search finds an out-of-scope value")
	}
	if v, ok := f.stored(inChild, model); !ok || v != "XJ-900" {
		t.Fatal("value lost on moving out of scope")
	}
	f.db.Exec(`UPDATE products SET category_id=$1 WHERE id=$2`, f.child, inChild) //nolint:errcheck
	if ids, _ := f.s.MatchProductIDs(ctx, "xj-900"); !slices.Contains(ids, inChild) {
		t.Fatal("search misses the value once back in scope")
	}
	if rows := f.mine(f.s.TillRows(ctx, inChild)); len(rows) != 1 || rows[0].Value != "XJ-900" {
		t.Fatalf("till rows: %+v", rows)
	}
}

func TestMissing(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a := f.product(t, "A", f.parent)
	b := f.product(t, "B", f.child)
	off := f.product(t, "Off", f.parent)
	f.db.Exec(`UPDATE products SET is_active=false WHERE id=$1`, off) //nolint:errcheck
	model := f.field(t, Field{Label: "Model", Type: "text", Required: true}, f.parent)
	withDefault := f.field(t, Field{Label: "Grade", Type: "text", Required: true, DefaultValue: "A"}, f.parent)
	yes := f.field(t, Field{Label: "Waterproof", Type: "bool", Required: true}, f.parent)
	f.set(t, a, model, "M1")

	stats, err := f.s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats[model.ID].Missing != 1 || stats[withDefault.ID].Missing != 0 || stats[yes.ID].Missing != 0 {
		t.Fatalf("missing: model %d grade %d yes/no %d", stats[model.ID].Missing, stats[withDefault.ID].Missing, stats[yes.ID].Missing)
	}
	rows, total, err := f.s.Grid(ctx, GridQuery{FieldIDs: []int64{model.ID}, CategoryID: f.parent, MissingOnly: true, Limit: 50})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != b {
		t.Fatalf("grid missing only: %d %+v %v", total, rows, err)
	}
	if v, ok := rows[0].Values[model.ID]; !ok || v != nil {
		t.Fatalf("grid cell for a product reading the default: %v %v", v, ok)
	}
}

func TestValues(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.product(t, "P", f.parent)
	grade := f.field(t, Field{Label: "Grade", Type: "text", DefaultValue: "A"})
	yes := f.field(t, Field{Label: "Boxed", Type: "bool", DefaultValue: "1"})
	qty := f.field(t, Field{Label: "Pack", Type: "number"})
	pick := f.field(t, Field{Label: "Size", Type: "select", Options: []string{"S", "M"}})
	day := f.field(t, Field{Label: "Made", Type: "date"})

	// A chosen value equal to the default is kept (a default change can't move it).
	f.set(t, p, grade, "A")
	if v, ok := f.stored(p, grade); !ok || v != "A" {
		t.Fatal("a chosen value equal to the default was not kept")
	}
	// Yes/no unticked against a Yes default stays No.
	f.set(t, p, yes, "no")
	if v, ok := f.stored(p, yes); !ok || v != "" {
		t.Fatalf("unticked yes/no: %q %v", v, ok)
	}
	saved, errs, err := f.s.SetValues(ctx, []Cell{
		{ProductID: p, FieldID: qty.ID, Value: "1,5"},
		{ProductID: p, FieldID: pick.ID, Value: "XL"},
		{ProductID: p, FieldID: day.ID, Value: "2026-13-01"},
		{ProductID: p, FieldID: pick.ID, Value: "m"},
	})
	if err != nil || saved != 1 || len(errs) != 3 {
		t.Fatalf("per-cell errors: saved %d errs %+v %v", saved, errs, err)
	}
	if v, _ := f.stored(p, pick); v != "M" {
		t.Fatalf("dropdown matched case-insensitively to its option: %q", v)
	}
}

func TestDefaultChange(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	untouched := f.product(t, "Untouched", f.parent)
	chose := f.product(t, "Chose", f.parent)
	grade := f.field(t, Field{Label: "Grade", Type: "text", DefaultValue: "A"}, f.parent)
	f.set(t, chose, grade, "B")

	g := grade
	g.DefaultValue = "C"
	if err := f.s.UpdateField(ctx, g, []int64{f.parent}, UpdateOpts{DefaultFor: "new"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := formValue(t, f.s, untouched, f.parent, grade); v != "A" {
		t.Fatalf("only new: an existing product changed to %q", v)
	}
	if v, _ := formValue(t, f.s, chose, f.parent, grade); v != "B" {
		t.Fatalf("only new: a chosen value changed to %q", v)
	}
	fresh := f.product(t, "Fresh", f.parent)
	if v, _ := formValue(t, f.s, fresh, f.parent, grade); v != "C" {
		t.Fatalf("a new product reads %q, want the new default", v)
	}

	g.DefaultValue = "D"
	if err := f.s.UpdateField(ctx, g, []int64{f.parent}, UpdateOpts{DefaultFor: "all"}); err != nil {
		t.Fatal(err)
	}
	if v, _ := formValue(t, f.s, fresh, f.parent, grade); v != "D" {
		t.Fatalf("all: an untouched product reads %q", v)
	}
	if v, _ := formValue(t, f.s, untouched, f.parent, grade); v != "A" {
		t.Fatalf("all: the value kept by the earlier 'only new' changed to %q", v)
	}
}

func TestTypeChange(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p1 := f.product(t, "P1", f.parent)
	p2 := f.product(t, "P2", f.parent)
	note := f.field(t, Field{Label: "Note", Type: "text"})
	f.set(t, p1, note, "12")
	f.set(t, p2, note, "twelve")

	im, err := f.s.Impact(ctx, note.ID, "number", nil)
	if err != nil || im.Lossy != 1 || im.Total != 2 {
		t.Fatalf("impact: %+v %v", im, err)
	}
	if im, _ := f.s.Impact(ctx, note.ID, "textarea", nil); im.Lossy != 0 {
		t.Fatalf("text → long text loses %d", im.Lossy)
	}
	n := note
	n.Type = "number"
	var loss *ErrDataLoss
	if err := f.s.UpdateField(ctx, n, nil, UpdateOpts{}); !errors.As(err, &loss) || loss.Lossy != 1 {
		t.Fatalf("lossy change without a confirm: %v", err)
	}
	if got, _ := f.s.Field(ctx, note.ID); got.Type != "text" {
		t.Fatal("refused change still changed the type")
	}
	if err := f.s.UpdateField(ctx, n, nil, UpdateOpts{ConfirmLoss: true}); err != nil {
		t.Fatal(err)
	}
	if v, ok := f.stored(p1, note); !ok || v != "12" {
		t.Fatal("a fitting value was dropped")
	}
	if _, ok := f.stored(p2, note); ok {
		t.Fatal("a value that doesn't fit was kept")
	}
}

func TestSearchMetaRows(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p := f.product(t, "P", f.parent)
	q := f.product(t, "Q", f.parent)
	grade := f.field(t, Field{Label: "Grade", Type: "text", DefaultValue: "Gold" + f.tag, Searchable: true, ShowAtTill: true, PrintOnLabel: true}, f.parent)
	wet := f.field(t, Field{Label: "Waterproof", Type: "bool", Searchable: true, ShowAtTill: true})
	f.set(t, p, wet, "yes")
	f.set(t, q, grade, "Silver"+f.tag)

	ids, _ := f.s.MatchProductIDs(ctx, "gold"+f.tag)
	if !slices.Contains(ids, p) || slices.Contains(ids, q) {
		t.Fatalf("search by the default: %v", ids)
	}
	if ids, _ := f.s.MatchProductIDs(ctx, f.tag+" waterproof"); !slices.Contains(ids, p) || slices.Contains(ids, q) {
		t.Fatalf("search a yes/no by label: %v", ids)
	}
	m, _ := f.s.MetaFor(ctx, []int64{p, q})
	if f.mineMeta(m[p]) != f.tag+" Grade: Gold"+f.tag+" · "+f.tag+" Waterproof" || f.mineMeta(m[q]) != f.tag+" Grade: Silver"+f.tag {
		t.Fatalf("meta: %q / %q", m[p], m[q])
	}
	if rows := f.mine(f.s.LabelRows(ctx, q)); len(rows) != 1 || rows[0].Value != "Silver"+f.tag {
		t.Fatalf("label rows: %+v", rows)
	}
	if rows := f.mine(f.s.TillRows(ctx, q)); len(rows) != 1 {
		t.Fatalf("till rows skip a No yes/no: %+v", rows)
	}
}
