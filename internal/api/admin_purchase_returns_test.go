package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminPurchaseReturns(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Return Supplier %d", time.Now().UnixNano())}, ck))
	other := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Return Other %d", time.Now().UnixNano())}, ck))
	receive := func(s int64, lines []StockLine, disc string) ReceiveResult {
		return decode[ReceiveResult](t, te.do(http.MethodPost, "/api/v1/admin/purchases/receive", PurchaseRequest{SupplierID: s, Discount: disc, DiscountType: "fixed", Lines: lines}, ck))
	}
	// An older bill first (so "oldest first" would pick it), then the one being returned:
	// 10 plain × 100 less 10% on the line, 2 free, and 100 off the bill → 800 for 12 units.
	older := receive(sup.ID, []StockLine{{ProductID: f.scarceID, Quantity: "1", CostPrice: "50", DiscountType: "fixed"}}, "")
	got := receive(sup.ID, []StockLine{{ProductID: f.plainID, Quantity: "10", FreeQty: "2", CostPrice: "100", Discount: "10", DiscountType: "percent"},
		{ProductID: f.lotID, Quantity: "5", CostPrice: "40", DiscountType: "fixed"}}, "100")
	other1 := receive(other.ID, []StockLine{{ProductID: f.plainID, Quantity: "3", CostPrice: "90", DiscountType: "fixed"}}, "")

	rl := decode[ReturnableList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", got.PurchaseID), nil, ck))
	if len(rl.Lines) != 2 {
		t.Fatalf("returnable: %+v", rl)
	}
	plain := rl.Lines[0]
	// 10×100 −10% = 900; bill 100 off 1100 net → 900 × 1000/1100 = 818.18 over 12 units = 68.18.
	if plain.ProductID != f.plainID || plain.Left != "12" || plain.Cost != "68.18" || plain.ListCost != "100.00" {
		t.Fatalf("plain line: %+v", plain)
	}

	post := func(req PurchaseReturnRequest) (int, PurchaseReturnRow) {
		rec := te.do(http.MethodPost, "/api/v1/admin/purchase-returns", req, ck)
		if rec.Code != http.StatusCreated {
			return rec.Code, PurchaseReturnRow{}
		}
		return rec.Code, decode[PurchaseReturnRow](t, rec)
	}
	// Not from this delivery: another supplier's lot, or a line without its lot.
	otherLot := decode[ReturnableList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", other1.PurchaseID), nil, ck)).Lines[0]
	if code, _ := post(PurchaseReturnRequest{SupplierID: sup.ID, PurchaseID: got.PurchaseID, Lines: []PurchaseReturnItem{{ProductID: f.plainID, Quantity: "1", BatchID: otherLot.BatchID}}}); code != http.StatusUnprocessableEntity {
		t.Fatalf("another delivery's lot: %d", code)
	}
	// Without a delivery: only stock that didn't come on one (old / opening stock),
	// at that lot's own cost — a typed price is ignored, delivery lots are refused.
	if code, _ := post(PurchaseReturnRequest{SupplierID: sup.ID, Lines: []PurchaseReturnItem{{ProductID: f.plainID, Quantity: "1", CostPrice: "100"}}}); code != http.StatusUnprocessableEntity {
		t.Fatalf("no delivery and no lot: %d", code)
	}
	if code, _ := post(PurchaseReturnRequest{SupplierID: sup.ID, Lines: []PurchaseReturnItem{{ProductID: f.plainID, Quantity: "1", BatchID: plain.BatchID}}}); code != http.StatusUnprocessableEntity {
		t.Fatalf("a delivery's lot without its delivery: %d", code)
	}
	var old LotRow
	for _, l := range decode[ProductLots](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/stock/lots/%d", f.scarceID), nil, ck)).Lots {
		if !l.FromDelivery {
			old = l // seedTill's lot: stock from before any delivery, cost 50
		}
	}
	code, r0 := post(PurchaseReturnRequest{SupplierID: sup.ID, Reason: "old stock", Lines: []PurchaseReturnItem{{ProductID: f.scarceID, Quantity: "1", CostPrice: "999", BatchID: old.ID}}})
	if code != http.StatusCreated || r0.Total != "50.00" || r0.PurchaseID != 0 {
		t.Fatalf("old stock return: %d %+v", code, r0)
	}
	if op := decode[PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d", older.PurchaseID), nil, ck)); op.Credited != "50.00" {
		t.Fatalf("old stock credit goes to the oldest bill: %+v", op.PurchaseRow)
	}
	if code, _ := post(PurchaseReturnRequest{SupplierID: sup.ID, PurchaseID: got.PurchaseID, Lines: []PurchaseReturnItem{{ProductID: f.plainID, Quantity: "13", BatchID: plain.BatchID}}}); code < 400 {
		t.Fatalf("more than came in: %d", code)
	}

	// Return 4 at the discounted cost (a typed price is ignored) → 272.72, credited to THIS bill.
	code, r := post(PurchaseReturnRequest{SupplierID: sup.ID, PurchaseID: got.PurchaseID, Reference: "DN-1", Reason: "damaged",
		Lines: []PurchaseReturnItem{{ProductID: f.plainID, Quantity: "4", CostPrice: "100", BatchID: plain.BatchID}}})
	if code != http.StatusCreated || r.Total != "272.72" || r.PurchaseID != got.PurchaseID || r.SupplierName != sup.Name {
		t.Fatalf("return: %d %+v", code, r)
	}
	gp := decode[PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d", got.PurchaseID), nil, ck))
	op := decode[PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d", older.PurchaseID), nil, ck))
	if gp.Credited != "272.72" || op.Credited != "50.00" {
		t.Fatalf("credited: returned bill %s, older bill %s", gp.Credited, op.Credited)
	}
	if left := decode[ReturnableList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", got.PurchaseID), nil, ck)).Lines[0].Left; left != "8" {
		t.Fatalf("left after return: %s", left)
	}
	d := decode[PurchaseReturnDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchase-returns/%d", r.ID), nil, ck))
	if len(d.Lines) != 1 || d.Lines[0].CostPrice != "68.18" || d.Reason != "damaged" || d.InvoiceNo != "" {
		t.Fatalf("detail: %+v", d)
	}
	if l := decode[PurchaseReturnList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchase-returns?supplier=%d", sup.ID), nil, ck)); l.Total != 2 {
		t.Fatalf("list: %+v", l)
	}
}

// Free units + line discount + bill discount all reach the return price, and
// sending the WHOLE delivery back clears exactly what its bill was:
//
//	A: 10 @ 100 + 2 free, 10% line discount → line 900
//	B:  5 @ 200                             → line 1000
//	bill: Rs 190 off the 1900               → owed 1710
//
// The bill discount spreads ×0.9, so A's lot is 900×0.9/12 = 67.50, B's 180.
func TestPurchaseReturnFromMixedDiscountDelivery(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Mixed Supplier %d", time.Now().UnixNano())}, ck))
	receive := func() (int64, ReturnableLine, ReturnableLine) {
		got := decode[ReceiveResult](t, te.do(http.MethodPost, "/api/v1/admin/purchases/receive", PurchaseRequest{SupplierID: sup.ID, Discount: "190", DiscountType: "fixed", Lines: []StockLine{
			{ProductID: f.plainID, Quantity: "10", FreeQty: "2", CostPrice: "100", Discount: "10", DiscountType: "percent"},
			{ProductID: f.lotID, Quantity: "5", CostPrice: "200", DiscountType: "fixed"},
		}}, ck))
		rl := decode[ReturnableList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", got.PurchaseID), nil, ck))
		if len(rl.Lines) != 2 || rl.Lines[0].Cost != "67.50" || rl.Lines[1].Cost != "180.00" {
			t.Fatalf("lot costs A/B, want 67.50/180.00: %+v", rl.Lines)
		}
		return got.PurchaseID, rl.Lines[0], rl.Lines[1]
	}
	post := func(pid int64, lines ...PurchaseReturnItem) PurchaseReturnRow {
		rec := te.do(http.MethodPost, "/api/v1/admin/purchase-returns", PurchaseReturnRequest{SupplierID: sup.ID, PurchaseID: pid, Lines: lines}, ck)
		if rec.Code != http.StatusCreated {
			t.Fatalf("return: %d %s", rec.Code, rec.Body)
		}
		return decode[PurchaseReturnRow](t, rec)
	}

	pid, a, b := receive()
	if r := post(pid, PurchaseReturnItem{ProductID: f.plainID, Quantity: "3", BatchID: a.BatchID}, PurchaseReturnItem{ProductID: f.lotID, Quantity: "2", BatchID: b.BatchID}); r.Total != "562.50" {
		t.Fatalf("credited %s, want 562.50 (3×67.50 + 2×180)", r.Total)
	}

	pid, a, b = receive()
	if r := post(pid, PurchaseReturnItem{ProductID: f.plainID, Quantity: "12", BatchID: a.BatchID}, PurchaseReturnItem{ProductID: f.lotID, Quantity: "5", BatchID: b.BatchID}); r.Total != "1710.00" {
		t.Fatalf("whole delivery credited %s, want the bill's 1710", r.Total)
	}
	if p := decode[PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d", pid), nil, ck)); p.Credited != "1710.00" {
		t.Fatalf("delivery bill credited %s, want 1710 (fully cleared)", p.Credited)
	}
}
