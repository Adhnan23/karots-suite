package aicatalog

import (
	"context"
	"encoding/json"
	"testing"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

func planOf(t *testing.T, js string) CleanupPlan {
	t.Helper()
	var p CleanupPlan
	if err := json.Unmarshal([]byte(js), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func cat(t *testing.T, db *sqlx.DB, name string, parent any) int64 {
	var id int64
	mustExec(t, db, &id, `INSERT INTO categories (name, parent_id) VALUES ($1, $2) RETURNING id`, name, parent)
	return id
}

func prod(t *testing.T, db *sqlx.DB, name string, cat int64) int64 {
	var id int64
	mustExec(t, db, &id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, $2, (SELECT id FROM units LIMIT 1), 1, 2) RETURNING id`, name, cat)
	return id
}

func str(t *testing.T, db *sqlx.DB, q string, args ...any) string {
	t.Helper()
	var s string
	if err := db.Get(&s, q, args...); err != nil {
		t.Fatal(err)
	}
	return s
}

// Unknown ids, a self-merge, a cycle, merging into a descendant and a product
// rename onto another product's name are skipped — never a failed transaction.
func TestApplySkipsUnsafeOps(t *testing.T) {
	db := freshDB(t, "aicatalog_apply_skip")
	ctx := context.Background()
	st := NewStore(db)
	a := cat(t, db, "Tools", nil)
	b := cat(t, db, "Hand tools", a)
	p1 := prod(t, db, "Hammer", b)
	prod(t, db, "Spanner", b)
	plan := planOf(t, `{"renames":[{"id":999999,"to":"x"},{"id":`+itoa(a)+`,"to":"  "}],
		"reparents":[{"id":`+itoa(a)+`,"parent_id":`+itoa(b)+`},{"id":`+itoa(b)+`,"parent_id":`+itoa(b)+`}],
		"merges":[{"from":`+itoa(a)+`,"into":`+itoa(a)+`},{"from":`+itoa(a)+`,"into":`+itoa(b)+`},{"from":999999,"into":`+itoa(a)+`}],
		"moves":[{"product_id":999999,"category_id":`+itoa(a)+`},{"product_id":`+itoa(p1)+`,"category_id":999999}],
		"uncategorized":[{"product_id":999999,"category":"Soap"}],
		"names":[{"product_id":`+itoa(p1)+`,"to":"spanner"}]}`)
	summary, run, err := st.ApplyPlan(ctx, plan, 0, 0)
	if err != nil || run != 0 || summary != "No changes applied." {
		t.Fatalf("%q %d %v", summary, run, err)
	}
	if str(t, db, `SELECT name FROM products WHERE id=$1`, p1) != "Hammer" || count(t, db, `SELECT count(*) FROM categories WHERE id IN ($1,$2) AND (id=$1 AND parent_id IS NULL OR parent_id=$1)`, a, b) != 2 {
		t.Fatal("something unsafe was applied")
	}
}

// A whole clean-up over two chunks reverts as one: ids, names, parents and
// placements come back and a category it created is deleted.
func TestApplyUndoRoundTrip(t *testing.T) {
	db := freshDB(t, "aicatalog_apply_undo")
	ctx := context.Background()
	st := NewStore(db)
	drinks, drink := cat(t, db, "drinks", nil), cat(t, db, "Drink", nil)
	snacks, biscuits := cat(t, db, "Snacks", nil), cat(t, db, "Biscuits", nil)
	uncat := cat(t, db, "Uncategorized", nil)
	coke, soap, mari := prod(t, db, "Coke 1L", drink), prod(t, db, "Sunlight soap", uncat), prod(t, db, "marie biscuit", snacks)
	plan := planOf(t, `{"renames":[{"id":`+itoa(drinks)+`,"to":"Drinks"}],
		"reparents":[{"id":`+itoa(biscuits)+`,"parent_id":`+itoa(snacks)+`}],
		"merges":[{"from":`+itoa(drink)+`,"into":`+itoa(drinks)+`}],
		"moves":[{"product_id":`+itoa(mari)+`,"category_id":`+itoa(biscuits)+`}],
		"uncategorized":[{"product_id":`+itoa(soap)+`,"category":"Household > Soap"}]}`)
	summary, run, err := st.ApplyPlan(ctx, plan, 0, 0)
	if err != nil || run == 0 {
		t.Fatalf("%q %v", summary, err)
	}
	want := "1 category renamed, 1 category moved, 1 category merged, 1 product moved, 1 uncategorized placed"
	if summary != want {
		t.Errorf("summary %q", summary)
	}
	if str(t, db, `SELECT c.name FROM products p JOIN categories c ON c.id = p.category_id WHERE p.id=$1`, soap) != "Soap" ||
		count(t, db, `SELECT count(*) FROM categories WHERE id=$1`, drink) != 0 ||
		count(t, db, `SELECT category_id FROM products WHERE id=$1`, coke) != int(drinks) {
		t.Fatal("chunk 1 not applied")
	}
	// Chunk 2 joins the same run.
	summary2, run2, err := st.ApplyPlan(ctx, planOf(t, `{"names":[{"product_id":`+itoa(mari)+`,"to":"Marie Biscuit"}]}`), 0, run)
	if err != nil || run2 != run || summary2 != want+"; 1 product renamed" {
		t.Fatalf("chunk 2: %q %d %v", summary2, run2, err)
	}
	if r, _ := st.LatestRun(ctx); r == nil || r.ID != run {
		t.Fatalf("last run %+v", r)
	}
	if _, err := st.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		q    string
		args []any
		want int
	}{
		{`SELECT count(*) FROM categories WHERE id=$1 AND name='Drink'`, []any{drink}, 1},
		{`SELECT count(*) FROM categories WHERE id=$1 AND name='drinks'`, []any{drinks}, 1},
		{`SELECT count(*) FROM categories WHERE id=$1 AND parent_id IS NULL`, []any{biscuits}, 1},
		{`SELECT count(*) FROM products WHERE id=$1 AND category_id=$2`, []any{coke, drink}, 1},
		{`SELECT count(*) FROM products WHERE id=$1 AND category_id=$2 AND name='marie biscuit'`, []any{mari, snacks}, 1},
		{`SELECT count(*) FROM products WHERE id=$1 AND category_id=$2`, []any{soap, uncat}, 1},
		{`SELECT count(*) FROM categories WHERE name IN ('Household','Soap')`, nil, 0},
	}
	for _, c := range checks {
		if n := count(t, db, c.q, c.args...); n != c.want {
			t.Errorf("%s %v = %d, want %d", c.q, c.args, n, c.want)
		}
	}
	if _, err := st.Revert(ctx); err != errNothingToUndo {
		t.Fatalf("second undo: %v", err)
	}
	// A fresh category after the undo doesn't collide with a restored id.
	cat(t, db, "After", nil)
}

func TestEmptyCategories(t *testing.T) {
	db := freshDB(t, "aicatalog_empty")
	ctx := context.Background()
	st := NewStore(db)
	empty, used := cat(t, db, "Nothing here", nil), cat(t, db, "Has one", nil)
	parent := cat(t, db, "Parent only", nil)
	cat(t, db, "Its child", parent)
	prod(t, db, "Thing", used)
	list, err := st.EmptyCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, c := range list {
		got[c.ID] = true
	}
	if !got[empty] || got[used] || got[parent] {
		t.Fatalf("empty list %+v", list)
	}
	if n, err := st.DeleteEmpty(ctx, []int64{empty, used, parent}); err != nil || n != 1 {
		t.Fatalf("deleted %d %v", n, err)
	}
}

func itoa(i int64) string { b, _ := json.Marshal(i); return string(b) }

// A plugin that keeps category ids (CategoryMerger) is asked on merge, and on
// undo gets back exactly the note it returned.
func TestApplyMergeAsksPlugins(t *testing.T) {
	db := freshDB(t, "aicatalog_apply_plugins")
	ctx := context.Background()
	st := NewStore(db)
	from, into := cat(t, db, "Bearing", nil), cat(t, db, "Bearings", nil)
	var merged [][2]int64
	var unmerged []string
	(&plugin.Registry{}).AddCategoryMerger(plugin.CategoryMerger{Key: "test-scope",
		Merge: func(_ context.Context, _ *sqlx.Tx, f, i int64) (json.RawMessage, error) {
			if f != from {
				return nil, nil // another test's merge: nothing of ours
			}
			merged = append(merged, [2]int64{f, i})
			return json.RawMessage(`{"fields":[7]}`), nil
		},
		Unmerge: func(_ context.Context, _ *sqlx.Tx, note json.RawMessage) error {
			unmerged = append(unmerged, string(note))
			return nil
		}})
	if _, _, err := st.ApplyPlan(ctx, planOf(t, `{"merges":[{"from":`+itoa(from)+`,"into":`+itoa(into)+`}]}`), 0, 0); err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || merged[0] != [2]int64{from, into} {
		t.Fatalf("merge calls %v", merged)
	}
	if _, err := st.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if len(unmerged) != 1 || unmerged[0] != `{"fields":[7]}` {
		t.Fatalf("unmerge got %v", unmerged)
	}
}
