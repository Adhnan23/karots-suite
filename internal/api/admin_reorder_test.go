package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
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
