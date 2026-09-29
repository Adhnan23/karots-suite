package api

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestAdminCategories(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009511", "admin")
	sfx := strconv.FormatInt(time.Now().UnixNano(), 36)
	mk := func(name string, parent int64) int64 {
		t.Helper()
		rec := te.do(http.MethodPost, "/api/v1/admin/categories", CategoryRequest{Name: name + " " + sfx, ParentID: parent}, ck)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", name, rec.Code, rec.Body)
		}
		id := decode[CategoryQuickResult](t, rec).ID
		t.Cleanup(func() { te.db.Exec(`DELETE FROM categories WHERE id=$1`, id) }) //nolint:errcheck
		return id
	}
	food := mk("Food", 0)
	fruit := mk("Fruit", food)
	apple := mk("Apple", fruit)

	tree := decode[[]CategoryNode](t, te.do(http.MethodGet, "/api/v1/admin/categories", nil, ck))
	i := slices.IndexFunc(tree, func(n CategoryNode) bool { return n.ID == apple })
	if i < 0 || tree[i].Depth != 2 || !slices.Equal(tree[i].Path, []int64{food, fruit}) {
		t.Fatalf("apple node: %+v", tree[i])
	}

	// No loops: Food can't move under its own grandchild, nor itself.
	for _, p := range []int64{apple, food} {
		if rec := te.do(http.MethodPut, "/api/v1/admin/categories/"+strconv.FormatInt(food, 10), CategoryRequest{Name: "Food " + sfx, ParentID: p}, ck); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("loop under %d: %d %s", p, rec.Code, rec.Body)
		}
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/categories", CategoryRequest{Name: "a > b"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("'>' in name: %d", rec.Code)
	}

	// Quick create is find-or-create under the tapped parent.
	q1 := decode[CategoryQuickResult](t, te.do(http.MethodPost, "/api/v1/admin/categories/quick", CategoryRequest{Name: "Mango", ParentID: fruit}, ck))
	t.Cleanup(func() { te.db.Exec(`DELETE FROM categories WHERE id=$1`, q1.ID) }) //nolint:errcheck
	q2 := decode[CategoryQuickResult](t, te.do(http.MethodPost, "/api/v1/admin/categories/quick", CategoryRequest{Name: "Mango", ParentID: fruit}, ck))
	if q1.ID != q2.ID || q1.Depth != 2 {
		t.Fatalf("quick: %+v %+v", q1, q2)
	}

	// Deleting Fruit lifts Apple to the top level.
	if rec := te.do(http.MethodDelete, "/api/v1/admin/categories/"+strconv.FormatInt(fruit, 10), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	var parent *int64
	te.db.Get(&parent, `SELECT parent_id FROM categories WHERE id=$1`, apple) //nolint:errcheck
	if parent != nil {
		t.Fatalf("apple parent after delete: %v", *parent)
	}
}
