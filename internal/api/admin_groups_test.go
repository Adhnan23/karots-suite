package api

import (
	"net/http"
	"slices"
	"strconv"
	"testing"
)

func TestAdminGroups(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009541", "admin")
	mk := func(name string, parent int64) int64 {
		t.Helper()
		rec := te.do(http.MethodPost, "/api/v1/admin/groups", GroupRequest{Name: name, Emoji: "🥤", ParentID: parent}, ck)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body)
		}
		id := decode[Option](t, rec).ID
		t.Cleanup(func() { te.db.Exec(`DELETE FROM product_groups WHERE id=$1`, id) }) //nolint:errcheck
		return id
	}
	drinks := mk("T Drinks", 0)
	soda := mk("T Soda", drinks)
	base := "/api/v1/admin/groups/" + strconv.FormatInt(soda, 10)

	if rec := te.do(http.MethodPost, base+"/items", GroupItemRequest{ProductID: f.plainID, Emoji: "🥫"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("link: %d %s", rec.Code, rec.Body)
	}
	te.do(http.MethodPut, base+"/items/"+strconv.FormatInt(f.plainID, 10)+"/emoji", GroupItemRequest{Emoji: "🧃"}, ck)
	items := decode[[]GroupItem](t, te.do(http.MethodGet, base+"/items", nil, ck))
	if len(items) != 1 || items[0].Emoji != "🧃" {
		t.Fatalf("items: %+v", items)
	}
	tree := decode[[]GroupNode](t, te.do(http.MethodGet, "/api/v1/admin/groups", nil, ck))
	i := slices.IndexFunc(tree, func(n GroupNode) bool { return n.ID == soda })
	if i < 1 || tree[i].Depth != 1 || tree[i].ParentID != drinks || tree[i].ItemCount != 1 || tree[i-1].ID != drinks {
		t.Fatalf("tree: %+v", tree)
	}
	if rec := te.do(http.MethodPost, base+"/move?dir=sideways", nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad move: %d", rec.Code)
	}
	te.do(http.MethodDelete, base+"/items/"+strconv.FormatInt(f.plainID, 10), nil, ck)
	if items := decode[[]GroupItem](t, te.do(http.MethodGet, base+"/items", nil, ck)); len(items) != 0 {
		t.Fatalf("unlink: %+v", items)
	}
	// Disabling the parent disables the sub-group too; enabling brings both back.
	parent := "/api/v1/admin/groups/" + strconv.FormatInt(drinks, 10)
	te.do(http.MethodDelete, parent, nil, ck)
	var active bool
	te.db.Get(&active, `SELECT is_active FROM product_groups WHERE id=$1`, soda) //nolint:errcheck
	if active {
		t.Fatal("sub-group still active after its parent was disabled")
	}
	te.do(http.MethodPost, parent+"/activate", nil, ck)
	te.db.Get(&active, `SELECT is_active FROM product_groups WHERE id=$1`, soda) //nolint:errcheck
	if !active {
		t.Fatal("sub-group not back after enabling its parent")
	}
}
