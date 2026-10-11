package api

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/plugin"
)

func TestAdminReorder(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sfx := time.Now().UnixNano()
	a := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Reorder A %d", sfx)}, ck))
	b := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Reorder B %d", sfx)}, ck))
	// scarce: 1 on hand, reorder at 5 → no sales history → 2×5 − 1 = 9.
	te.db.Exec(`UPDATE products SET reorder_level=5, preferred_supplier_id=$1 WHERE id=$2`, a.ID, f.scarceID) //nolint:errcheck

	l := decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d", a.ID), nil, ck))
	if l.Total != 1 || l.Items[0].ProductID != f.scarceID || l.Items[0].Suggested != "9" || l.Items[0].FromSales || l.Items[0].SupplierID != a.ID {
		t.Fatalf("reorder list: %+v", l)
	}
	rec := te.do(http.MethodPost, "/api/v1/admin/purchases/draft", ReorderRequest{Lines: []ReorderLine{
		{ProductID: f.scarceID, SupplierID: a.ID, Quantity: "9", CostPrice: "50"},
		{ProductID: f.plainID, SupplierID: b.ID, Quantity: "4", CostPrice: "50"},
	}}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("build: %d %s", rec.Code, rec.Body)
	}
	if ids := decode[IDsResult](t, rec).IDs; len(ids) != 2 {
		t.Fatalf("one order per supplier: %v", ids)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/purchases/draft", ReorderRequest{Lines: []ReorderLine{{ProductID: f.plainID, Quantity: "1"}}}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("line without supplier: %d", rec.Code)
	}
}

// fakeCover is a stand-in plugin annotator (hooks are global, so it is
// registered once): ids in coverIDs read as covered, pullIDs are pulled in.
var (
	fakeCoverOnce sync.Once
	fakeCoverMu   sync.Mutex
	coverIDs      = map[int64]bool{}
	pullIDs       []int64
)

func useFakeCover() {
	fakeCoverOnce.Do(func() {
		plugin.NewRegistry(plugin.Core{}, plugin.NewMux(), nil).AddProductReorderAnnotator(plugin.ProductReorderAnnotator{
			Batch: func(_ context.Context, ids []int64) (map[int64]plugin.ReorderNote, error) {
				fakeCoverMu.Lock()
				defer fakeCoverMu.Unlock()
				out := map[int64]plugin.ReorderNote{}
				for _, id := range ids {
					if coverIDs[id] {
						out[id] = plugin.ReorderNote{Covered: true, Note: "covered by a twin"}
					}
				}
				return out, nil
			},
			LowMembers: func(context.Context) ([]int64, error) {
				fakeCoverMu.Lock()
				defer fakeCoverMu.Unlock()
				return pullIDs, nil
			},
		})
	})
}

// A plugin annotator notes rows; Hide covered items drops covered rows and pulls
// in the plugin's products before paging, and the filters still apply to them.
func TestAdminReorderAnnotated(t *testing.T) {
	useFakeCover()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Cover %d", time.Now().UnixNano())}, ck))
	te.db.Exec(`UPDATE products SET reorder_level=5, preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.scarceID) //nolint:errcheck
	te.db.Exec(`UPDATE products SET reorder_level=500, preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.lotID)  //nolint:errcheck
	te.db.Exec(`UPDATE products SET reorder_level=0, preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.plainID)  //nolint:errcheck
	fakeCoverMu.Lock()
	coverIDs[f.lotID], pullIDs = true, []int64{f.plainID, f.expiringID} // expiring has another supplier: filtered out
	fakeCoverMu.Unlock()
	t.Cleanup(func() { fakeCoverMu.Lock(); coverIDs, pullIDs = map[int64]bool{}, nil; fakeCoverMu.Unlock() })

	ids := func(l ReorderList) []int64 {
		var out []int64
		for _, r := range l.Items {
			out = append(out, r.ProductID)
		}
		return out
	}
	l := decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d", sup.ID), nil, ck))
	if !l.Annotated || l.Total != 2 || l.Hidden != 0 || !slices.Contains(ids(l), f.lotID) {
		t.Fatalf("annotated list: %+v", l)
	}
	for _, r := range l.Items {
		if r.ProductID == f.lotID && (!r.Covered || r.Note != "covered by a twin") {
			t.Fatalf("note: %+v", r)
		}
	}
	l = decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d&hideCovered=1", sup.ID), nil, ck))
	got := ids(l)
	if l.Total != 2 || l.Hidden != 1 || !slices.Contains(got, f.scarceID) || !slices.Contains(got, f.plainID) || slices.Contains(got, f.expiringID) {
		t.Fatalf("hide covered: %+v", l)
	}
	if l = decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d&hideCovered=1&page=2", sup.ID), nil, ck)); len(l.Items) != 0 || l.Total != 2 {
		t.Fatalf("page 2 is past the filtered rows: %+v", l)
	}
}
