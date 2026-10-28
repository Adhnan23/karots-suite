package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func sale(pid int64, qty string, pays ...PaymentRequest) SaleRequest {
	return SaleRequest{SaleType: "retail", DiscountType: "fixed",
		Items: []SaleItemRequest{{ProductID: pid, Quantity: qty, DiscountType: "fixed"}}, Payments: pays}
}

func TestTillCashSaleAndLotPrice(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	// A fresh phone each run: posting a sale anchors this user via RESTRICT FKs
	// (sales.cashier_id, stock_movements.user_id), so a fixed phone would
	// collide on the next run (same reason as Task 2's ruling).
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")

	rec := te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "150"}), ck)
	res := decode[SaleResult](t, rec)
	if rec.Code != 201 || res.ReceiptNo == "" || res.Total != "100.00" || res.ChangeGiven != "50.00" || len(res.Methods) != 1 {
		t.Fatalf("cash sale %d %+v", rec.Code, res)
	}

	// ?preview=1 answers with the slip as the paper shows it — no printer needed.
	pv := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/print?preview=1", res.ID), nil, ck)
	slip := decode[SlipPreview](t, pv)
	var text []string
	for _, l := range slip.Lines {
		text = append(text, l.Text)
	}
	if pv.Code != 200 || slip.Columns == 0 || !strings.Contains(strings.Join(text, "\n"), res.ReceiptNo) || !strings.Contains(strings.Join(text, "\n"), "Karots Suite") {
		t.Fatalf("sale preview %d %+v", pv.Code, slip)
	}

	lots := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.lotID), nil, ck))
	var dear Lot
	for _, l := range lots {
		if l.Price == "120.00" {
			dear = l
		}
	}
	req := sale(f.lotID, "1", PaymentRequest{Method: "cash", Amount: "120"})
	req.Items[0].BatchID = dear.BatchID
	if res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, ck)); res.Total != "120.00" {
		t.Fatalf("picked lot must price at 120: %+v", res)
	}

	// Keep: the customer leaves Rs 5 of a Rs 50 change behind.
	req = sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "150"})
	req.ChangeGiven = "45.00"
	if res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, ck)); res.ChangeGiven != "45.00" {
		t.Fatalf("kept change: %+v", res)
	}
}

func TestTillOversellNeedsConfirm(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", sale(f.scarceID, "2", PaymentRequest{Method: "cash", Amount: "500"}), ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("selling 2 of 1 without confirm must fail, got %d", rec.Code)
	}
	req := sale(f.scarceID, "2", PaymentRequest{Method: "cash", Amount: "500"})
	req.Items[0].AllowOversell = true
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", req, ck); rec.Code != 201 {
		t.Fatalf("found-at-till sale %d %s", rec.Code, rec.Body)
	}
}

func TestTillOverLimitIsGatedByFlag(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	cashier := te.login(t, "0770009403", "cashier")
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	cust := decode[CustomerCreated](t, te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Limit Cust", Phone: phone, CreditLimit: "100"}, cashier)).Customer

	req := sale(f.plainID, "2", PaymentRequest{Method: "credit", Amount: "200"})
	req.CustomerID = &cust.ID
	req.AllowOverLimit = true // a cashier without the flag: the server must strip this
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", req, cashier); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("cashier over limit must be refused, got %d", rec.Code)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/sales", req, admin)
	if res := decode[SaleResult](t, rec); rec.Code != 201 || res.CustomerBalanceAfter == nil {
		t.Fatalf("admin over-limit approval %d %s", rec.Code, rec.Body)
	}
}

func TestTillHeldSales(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009405", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "x", SaleType: "retail", Cart: "{not json"}, ck); rec.Code != 400 {
		t.Fatalf("bad cart %d", rec.Code)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "Kamal", SaleType: "retail", Discount: "10", Cart: `{"v":2,"lines":[],"billDiscountType":"percent"}`, ItemCount: 2, Total: "340"}, ck)
	held := decode[HeldSale](t, rec)
	if rec.Code != 201 || held.ID == 0 || held.Cart == "" {
		t.Fatalf("hold %d %+v", rec.Code, held)
	}
	list := decode[[]HeldSale](t, te.do(http.MethodGet, "/api/v1/till/held", nil, ck))
	if len(list) != 1 || list[0].Label != "Kamal" || list[0].Discount != "10.00" {
		t.Fatalf("list %+v", list)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/till/held/%d", held.ID), nil, ck); rec.Code != 204 {
		t.Fatalf("delete %d", rec.Code)
	}
	if list := decode[[]HeldSale](t, te.do(http.MethodGet, "/api/v1/till/held", nil, ck)); len(list) != 0 {
		t.Fatalf("after delete %+v", list)
	}
}

func TestTillPrintUnknownSale(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009406", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/sales/999999999/print", nil, ck); rec.Code != 404 {
		t.Fatalf("unknown sale print %d", rec.Code)
	}
}
