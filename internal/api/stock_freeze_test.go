package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStockFreezeAPI(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	ak := te.login(t, fmt.Sprintf("07%08d", (time.Now().UnixNano()+1)%100_000_000), "admin")
	t.Cleanup(func() {
		te.db.Exec(`UPDATE products SET stock_frozen = false, stock_frozen_at = NULL`) //nolint:errcheck
		te.db.Exec(`UPDATE settings SET freeze_new_products = false`)                  //nolint:errcheck
	})
	onHand := func(id int64) string {
		var q string
		te.db.Get(&q, `SELECT quantity::float8::text FROM stock WHERE product_id=$1`, id) //nolint:errcheck
		return q
	}
	ok := func(what string, code, want int, body fmt.Stringer) {
		t.Helper()
		if code != want {
			t.Fatalf("%s: %d %s", what, code, body)
		}
	}

	// A cashier can't freeze; the admin can.
	rec := te.do(http.MethodPost, "/api/v1/admin/stock/freeze", FreezeRequest{ProductIDs: []int64{f.plainID}}, ck)
	if rec.Code < 400 {
		t.Fatalf("cashier froze: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/stock/freeze-shop", nil, ck); rec.Code < 400 {
		t.Fatalf("cashier froze the shop: %d", rec.Code)
	}
	res := decode[FreezeResultDTO](t, te.do(http.MethodPost, "/api/v1/admin/stock/freeze", FreezeRequest{ProductIDs: []int64{f.plainID}}, ak))
	if res.Frozen != 1 || len(res.Refused) != 0 {
		t.Fatalf("freeze: %+v", res)
	}

	// The till sees it frozen.
	p := decode[TillProduct](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d", f.plainID), nil, ck))
	if !p.StockFrozen {
		t.Fatal("till product not frozen")
	}
	list := decode[FrozenListDTO](t, te.do(http.MethodGet, "/api/v1/till/frozen", nil, ck)).Products
	listed := false
	for _, r := range list {
		listed = listed || r.ID == f.plainID
	}
	if !listed {
		t.Fatalf("frozen list %+v", list)
	}

	// Write-off from the shelf leaves stock alone; to the shelf takes it out.
	start := onHand(f.plainID)
	rec = te.do(http.MethodPost, "/api/v1/till/write-off", WriteOffRequest{ProductID: f.plainID, Quantity: "1", FromShelf: true}, ck)
	ok("write-off", rec.Code, http.StatusNoContent, rec.Body)
	if onHand(f.plainID) != start {
		t.Fatalf("shelf write-off moved stock: %s → %s", start, onHand(f.plainID))
	}
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/stock/%d/to-shelf", f.plainID), ToShelfRequest{Quantity: "2"}, ck)
	ok("to shelf", rec.Code, http.StatusNoContent, rec.Body)
	if onHand(f.plainID) != "8" {
		t.Fatalf("to shelf: %s", onHand(f.plainID))
	}
	// A cashier unfreezes with the shelf count.
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/stock/%d/unfreeze", f.plainID), UnfreezeRequest{ShelfQty: "-1"}, ck)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("negative shelf: %d", rec.Code)
	}
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/stock/%d/unfreeze", f.plainID), UnfreezeRequest{ShelfQty: "3"}, ck)
	ok("unfreeze", rec.Code, http.StatusNoContent, rec.Body)
	if onHand(f.plainID) != "11" {
		t.Fatalf("unfreeze: %s", onHand(f.plainID))
	}

	// The shop switch.
	rec = te.do(http.MethodPost, "/api/v1/admin/stock/freeze-shop", nil, ak)
	ok("freeze shop", rec.Code, http.StatusOK, rec.Body)
	fl := decode[FrozenListDTO](t, te.do(http.MethodGet, "/api/v1/admin/stock/frozen", nil, ak))
	if len(fl.Products) < 2 || !fl.ShopFrozen {
		t.Fatalf("frozen after freeze-shop: %d %v", len(fl.Products), fl.ShopFrozen)
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/stock/unfreeze-shop", nil, ak)
	ok("unfreeze shop", rec.Code, http.StatusOK, rec.Body)
	if fl := decode[FrozenListDTO](t, te.do(http.MethodGet, "/api/v1/admin/stock/frozen", nil, ak)); len(fl.Products) != 0 || fl.ShopFrozen {
		t.Fatalf("still frozen: %d %v", len(fl.Products), fl.ShopFrozen)
	}
}

// A sales person may unfreeze (the shelf count), mid-sale too.
func TestSalesPersonUnfreezes(t *testing.T) {
	te, s, _, _, _ := salesFixture(t)
	f := te.seedTill(t)
	te.db.MustExec(`UPDATE products SET stock_frozen = true WHERE id = $1`, f.plainID)
	t.Cleanup(func() { te.db.MustExec(`UPDATE products SET stock_frozen = false WHERE id = $1`, f.plainID) })
	rec := te.on("dev-s", http.MethodPost, fmt.Sprintf("/api/v1/till/stock/%d/unfreeze", f.plainID), UnfreezeRequest{ShelfQty: "2"}, s)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("sales person unfreeze: %d %s", rec.Code, rec.Body)
	}
}
