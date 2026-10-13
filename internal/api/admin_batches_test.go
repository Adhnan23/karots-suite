package api

import (
	"net/http"
	"slices"
	"testing"
)

func TestAdminBatches(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009561"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "admin")
	t.Cleanup(clean)

	v := decode[BatchesView](t, te.do(http.MethodGet, "/api/v1/admin/stock/batches", nil, ck))
	i := slices.IndexFunc(v.Groups, func(g BatchGroup) bool { return g.ProductID == f.expiringID })
	if i < 0 || v.Groups[i].Expired != 1 || len(v.Groups[i].Lots) != 2 || v.Groups[0].Expired == 0 {
		t.Fatalf("expired group first: %+v", v.Groups)
	}

	// One lot: shop use from the chosen lot.
	if rec := te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.plainID, Quantity: "1", Reason: "own_use"}, ck); rec.Code != http.StatusOK {
		t.Fatalf("consume: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/stock/damage", ConsumeRequest{ProductID: f.plainID, Quantity: "1", Reason: "gift"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown reason: %d", rec.Code)
	}

	r := decode[WriteOffResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/expired/write-off-all", nil, ck))
	if r.Lots < 1 {
		t.Fatalf("wrote off %d", r.Lots)
	}
	var left float64
	te.db.Get(&left, `SELECT quantity FROM stock WHERE product_id=$1`, f.expiringID) //nolint:errcheck
	if left != 3 {
		t.Fatalf("expiring product left %v, want the 3 unexpired", left)
	}
}
