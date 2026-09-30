package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminPurchases(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sfx := time.Now().UnixNano()
	var safe int64
	te.db.Get(&safe, `INSERT INTO lockers (name, kind, allow_negative) VALUES ($1, 'safe', true) RETURNING id`, fmt.Sprintf("T po safe %d", sfx)) //nolint:errcheck
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T PO Supplier %d", sfx)}, ck))
	onHand := func(id int64) string {
		var q string
		te.db.Get(&q, `SELECT quantity::float8::text FROM stock WHERE product_id=$1`, id) //nolint:errcheck
		return q
	}
	line := func(pid int64, qty, cost string) StockLine {
		return StockLine{ProductID: pid, Quantity: qty, CostPrice: cost, DiscountType: "fixed"}
	}

	// Order two lines, 10% off the bill: a draft, no stock, no payable.
	rec := te.do(http.MethodPost, "/api/v1/admin/purchases", PurchaseRequest{SupplierID: sup.ID, ExpectedDate: "2026-10-05", Discount: "10", DiscountType: "percent",
		Lines: []StockLine{line(f.plainID, "10", "50"), line(f.scarceID, "4", "200")}}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("order: %d %s", rec.Code, rec.Body)
	}
	po := decode[PurchaseRow](t, rec)
	if po.Status != "draft" || po.Total != "1170.00" || onHand(f.plainID) != "10" {
		t.Fatalf("draft: %+v, stock %s", po, onHand(f.plainID))
	}
	list := decode[PurchaseList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases?tab=orders&supplier=%d", sup.ID), nil, ck))
	if list.Total != 1 || list.Items[0].ExpectedDate != "2026-10-05" {
		t.Fatalf("orders: %+v", list)
	}
	path := fmt.Sprintf("/api/v1/admin/purchases/%d", po.ID)
	// Edit: drop the bill discount, 12 plain.
	if rec := te.do(http.MethodPut, path, PurchaseRequest{SupplierID: sup.ID, Lines: []StockLine{line(f.plainID, "12", "50"), line(f.scarceID, "4", "200")}}, ck); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	d := decode[PurchaseDetail](t, te.do(http.MethodGet, path, nil, ck))
	if d.Total != "1400.00" || len(d.Lines) != 2 || d.Lines[0].Ordered != "12" || d.Lines[0].CurCost != "50.00" || d.Supplier.Name != sup.Name {
		t.Fatalf("detail: %+v", d)
	}

	// Receive: 8 of 12 plain, all 4 scarce, a line that wasn't ordered; keep the rest on order; pay 300 from the safe.
	extra := line(f.lotID, "2", "60")
	recv := PurchaseRequest{SupplierID: sup.ID, InvoiceNo: "INV-77", DueDate: "2026-10-30", KeepRemainder: true,
		Lines: []StockLine{{ProductID: f.plainID, Ordered: "12", Quantity: "8", CostPrice: "50", DiscountType: "fixed"}, {ProductID: f.scarceID, Ordered: "4", Quantity: "4", CostPrice: "200", DiscountType: "fixed"}, extra},
		Pay:   AdminPayNow{Amount: "300", Method: "cash", Source: fmt.Sprintf("locker:%d", safe)}}
	rec = te.do(http.MethodPost, path+"/receive", recv, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("receive: %d %s", rec.Code, rec.Body)
	}
	if r := decode[ReceiveResult](t, rec); r.Total != "1320.00" || r.Paid != "300.00" || r.ReceiptID == nil {
		t.Fatalf("receive result: %+v", r)
	}
	if onHand(f.plainID) != "18" || onHand(f.lotID) != "12" {
		t.Fatalf("stock after receive: plain %s lot %s", onHand(f.plainID), onHand(f.lotID))
	}
	got := decode[PurchaseDetail](t, te.do(http.MethodGet, path, nil, ck))
	if got.Status == "draft" || got.InvoiceNo != "INV-77" || got.Balance != "1020.00" || got.DueDate != "2026-10-30" {
		t.Fatalf("received: %+v", got)
	}
	rest := decode[PurchaseList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases?tab=orders&supplier=%d", sup.ID), nil, ck))
	if rest.Total != 1 {
		t.Fatalf("remainder order: %+v", rest)
	}
	restD := decode[PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/%d", rest.Items[0].ID), nil, ck))
	if len(restD.Lines) != 1 || restD.Lines[0].Quantity != "4" {
		t.Fatalf("remainder lines: %+v", restD.Lines)
	}
	if rec := te.do(http.MethodDelete, path, nil, ck); rec.Code < 400 {
		t.Fatalf("deleting a received purchase: %d", rec.Code)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/purchases/%d", rest.Items[0].ID), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("delete draft: %d %s", rec.Code, rec.Body)
	}

	// Walk-in delivery, all on credit; then the list: received + unpaid + search by invoice.
	if rec := te.do(http.MethodPost, "/api/v1/admin/purchases/receive", PurchaseRequest{SupplierID: sup.ID, InvoiceNo: "WALK-1", Lines: []StockLine{line(f.plainID, "2", "50")}}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("receive now: %d %s", rec.Code, rec.Body)
	}
	got2 := decode[PurchaseList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases?supplier=%d&unpaid=1", sup.ID), nil, ck))
	if got2.Total != 2 {
		t.Fatalf("received unpaid: %+v", got2)
	}
	if l := decode[PurchaseList](t, te.do(http.MethodGet, "/api/v1/admin/purchases?q=WALK-1", nil, ck)); l.Total < 1 || l.Items[0].InvoiceNo != "WALK-1" {
		t.Fatalf("search: %+v", l)
	}
	if p := decode[[]PurchaseDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/purchases/print?ids=%d,%d", po.ID, got2.Items[0].ID), nil, ck)); len(p) != 2 {
		t.Fatalf("print: %d", len(p))
	}
}
