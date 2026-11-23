package battle

import (
	"fmt"
	"slices"
	"testing"
)

// CAT-1, 2: Delete empty categories lists the empty ones only, deletes the ticked
// ones, and keeps one that has products even when it is ticked.
func TestCAT1DeleteEmptyCategories(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	mk := func(name string, parent int64) int64 {
		var c struct{ ID int64 }
		s.must(t, s.admin.Do("POST", "/api/v1/admin/categories", map[string]any{"name": name, "parentId": parent}, &c))
		return c.ID
	}
	empty, keep := mk("Cat Empty", 0), mk("Cat Keep Me Empty", 0)
	full := mk("Cat Full", 0)
	s.stocked(t, "Cat Product", s.unitPcs, 0, map[string]any{"categoryId": full})
	parent := mk("Cat Parent", 0)
	child := mk("Cat Child", parent)
	list := func() []int64 {
		var l []struct{ ID int64 }
		s.must(t, s.admin.Do("GET", "/api/v1/admin/categories/empty", nil, &l))
		var ids []int64
		for _, c := range l {
			ids = append(ids, c.ID)
		}
		return ids
	}
	got := list()
	for _, id := range []int64{empty, keep, child} {
		if !slices.Contains(got, id) {
			t.Fatalf("CAT-1: %d not listed as empty (%v)", id, got)
		}
	}
	if slices.Contains(got, full) || slices.Contains(got, parent) {
		t.Fatalf("CAT-1: a category with products or a sub-category listed (%v)", got)
	}
	var r struct {
		Deleted int
		Kept    []string
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/categories/delete-empty", map[string]any{"ids": []int64{empty, child, full}}, &r))
	if r.Deleted != 2 || len(r.Kept) != 1 {
		t.Fatalf("CAT-2: %+v", r)
	}
	var n int
	s.must(t, s.db.Get(&n, fmt.Sprintf(`SELECT count(*) FROM categories WHERE id IN (%d, %d, %d)`, keep, full, parent)))
	if n != 3 {
		t.Fatalf("CAT-1: an unticked or full category went (%d left of 3)", n)
	}
	if !slices.Contains(list(), parent) {
		t.Fatal("CAT-2: the parent left empty isn't listed the next time")
	}
}
