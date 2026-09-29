package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/features/suppliers"
)

func TestCounterSuppliersPayAndRefund(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	phone := func() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }

	// The tab is behind the supplier flag, like Expenses.
	if rec := te.do(http.MethodGet, "/api/v1/till/suppliers", nil, te.login(t, phone(), "cashier")); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier without flag: %d", rec.Code)
	}
	ck := te.login(t, phone(), "manager")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)

	sup, err := suppliers.NewService(te.db).Create(context.Background(), suppliers.CreateInput{Name: fmt.Sprintf("T Supplier %d", time.Now().UnixNano()), OpeningBalance: "1000"})
	if err != nil {
		t.Fatal(err)
	}
	v := decode[SupplierView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d", sup.ID), nil, ck))
	if v.Owed != "1000.00" || v.OldDebt != "1000.00" || len(v.Invoices) != 0 {
		t.Fatalf("supplier view: %+v", v)
	}
	expected := func() string {
		return decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck)).Summary.Expected
	}

	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/pay", sup.ID), SupplierPayRequest{Amount: "400", Mode: "old", Method: "cash"}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID == nil {
		t.Fatalf("pay: %d %s", rec.Code, rec.Body)
	}
	if v := decode[SupplierView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d", sup.ID), nil, ck)); v.Owed != "600.00" {
		t.Fatalf("after paying 400: %+v", v)
	}
	if expected() != "600.00" {
		t.Fatalf("drawer after paying 400 cash: %s", expected())
	}
	// A card payment with no account picked never touches the counter: no slip.
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/pay", sup.ID), SupplierPayRequest{Amount: "100", Mode: "old", Method: "card"}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID != nil || expected() != "600.00" {
		t.Fatalf("card pay: %d %s, drawer %s", rec.Code, rec.Body, expected())
	}
	// Overpay 100 into an advance (500 owed, card again), then they hand 50 back in cash.
	te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/pay", sup.ID), SupplierPayRequest{Amount: "600", Mode: "old", Method: "card"}, ck)
	if v := decode[SupplierView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d", sup.ID), nil, ck)); v.Owed != "-100.00" {
		t.Fatalf("overpaid into an advance: %+v", v)
	}
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/refund", sup.ID), SupplierRefundRequest{Amount: "50", Method: "cash"}, ck)
	if rec.Code != http.StatusCreated || expected() != "650.00" {
		t.Fatalf("refund: %d %s, drawer %s", rec.Code, rec.Body, expected())
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/pay", sup.ID), SupplierPayRequest{Amount: "0"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("zero pay: %d", rec.Code)
	}
}

func TestCounterReceiveAndOrder(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "manager")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
	onHand := func(id int64) string {
		var q string
		te.db.Get(&q, `SELECT quantity::float8::text FROM stock WHERE product_id=$1`, id) //nolint:errcheck
		return q
	}

	rec := te.do(http.MethodPost, "/api/v1/till/suppliers", NewSupplierRequest{Name: fmt.Sprintf("T Walk-in %d", time.Now().UnixNano()), Phone: "0712345678"}, ck)
	sup := decode[SupplierRow](t, rec)
	if rec.Code != http.StatusCreated || sup.ID == 0 {
		t.Fatalf("quick supplier: %d %s", rec.Code, rec.Body)
	}

	// Walk-in delivery: 10 @ 60, 300 paid in cash now, the rest on credit.
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/receive", sup.ID), ReceiveRequest{InvoiceNo: "INV-1",
		Lines: []StockLine{{ProductID: f.plainID, Quantity: "10", CostPrice: "60", SellingPrice: "100"}}, Pay: PayNow{Amount: "300", Method: "cash"}}, ck)
	rr := decode[ReceiveResult](t, rec)
	if rec.Code != http.StatusCreated || rr.Total != "600.00" || rr.Paid != "300.00" || rr.ReceiptID == nil {
		t.Fatalf("receive: %d %s", rec.Code, rec.Body)
	}
	if onHand(f.plainID) != "20" {
		t.Fatalf("stock after delivery: %s", onHand(f.plainID))
	}
	v := decode[SupplierView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d", sup.ID), nil, ck))
	if v.Owed != "300.00" || len(v.Invoices) != 1 || v.Invoices[0].Balance != "300.00" {
		t.Fatalf("after delivery: %+v", v)
	}

	// Discounts: 10% off the line (600 → 540), then Rs 20 off the bill → 520.
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/receive", sup.ID), ReceiveRequest{Discount: "20", DiscountType: "fixed",
		Lines: []StockLine{{ProductID: f.plainID, Quantity: "10", CostPrice: "60", Discount: "10", DiscountType: "percent"}}}, ck)
	if rr := decode[ReceiveResult](t, rec); rec.Code != http.StatusCreated || rr.Total != "520.00" {
		t.Fatalf("discounted delivery: %d %s", rec.Code, rec.Body)
	}

	// An order without prices (cost falls back to the product's), then received.
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d/order", sup.ID), OrderRequest{Lines: []StockLine{{ProductID: f.scarceID, Quantity: "4"}}}, ck)
	ord := decode[SupplierOrder](t, rec)
	if rec.Code != http.StatusCreated || ord.ID == 0 {
		t.Fatalf("order: %d %s", rec.Code, rec.Body)
	}
	o := decode[OrderView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/orders/%d", ord.ID), nil, ck))
	if o.Status != "draft" || len(o.Lines) != 1 || o.Lines[0].Ordered != "4" || o.Lines[0].CostPrice != "50" {
		t.Fatalf("order view: %+v", o)
	}
	if v := decode[SupplierView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/suppliers/%d", sup.ID), nil, ck)); len(v.Orders) != 1 {
		t.Fatalf("order not listed: %+v", v.Orders)
	}
	o.Lines[0].Quantity = "3"
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/orders/%d/receive", ord.ID), ReceiveRequest{Lines: o.Lines}, ck)
	if rec.Code != http.StatusOK || onHand(f.scarceID) != "4" {
		t.Fatalf("receive order: %d %s, scarce stock %s", rec.Code, rec.Body, onHand(f.scarceID))
	}

	// Products first met on a delivery need real prices; a wanted item never takes them.
	if rec := te.do(http.MethodPost, "/api/v1/till/intake-products", IntakeProductRequest{Name: fmt.Sprintf("T new %d", time.Now().UnixNano())}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("intake without prices: %d", rec.Code)
	}
	rec = te.do(http.MethodPost, "/api/v1/till/intake-products", IntakeProductRequest{Name: fmt.Sprintf("T wanted %d", time.Now().UnixNano()), CostPrice: "999", Wanted: true}, ck)
	if p := decode[TillProduct](t, rec); rec.Code != http.StatusCreated || p.ID == 0 {
		t.Fatalf("wanted product: %d %s", rec.Code, rec.Body)
	}
}
