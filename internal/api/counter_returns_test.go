package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCounterReturn(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)

	res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "3", PaymentRequest{Method: "cash", Amount: "300"}), ck))
	found := decode[[]SaleSummary](t, te.do(http.MethodGet, "/api/v1/till/sales?q="+res.ReceiptNo, nil, ck))
	if len(found) != 1 || found[0].ID != res.ID {
		t.Fatalf("lookup by receipt: %+v", found)
	}
	v := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", res.ID), nil, ck))
	if len(v.Lines) != 1 || v.Lines[0].RefundUnit != "100.00" || v.Lines[0].ReturnedQty != "0" {
		t.Fatalf("sale view: %+v", v)
	}

	// One back on the shelf, one faulty (written off): both refunded in cash.
	line := v.Lines[0].ID
	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/return", res.ID), ReturnRequest{Reason: "wrong size", Lines: []ReturnLine{
		{SaleItemID: line, Quantity: "1"}, {SaleItemID: line, Quantity: "1", Disposition: "damage"},
	}}, ck)
	if rr := decode[ReturnResult](t, rec); rec.Code != 200 || rr.CashRefund != "200.00" {
		t.Fatalf("return: %d %s", rec.Code, rec.Body)
	}
	if st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck)); st.Summary.Expected != "1100.00" {
		t.Fatalf("drawer after refund: %+v", st.Summary) // 1000 float + 300 sale − 200 refund
	}
	var onHand string
	te.db.Get(&onHand, `SELECT quantity::float8::text FROM stock WHERE product_id=$1`, f.plainID) //nolint:errcheck
	if onHand != "8" {
		t.Fatalf("stock after return: %s (10 − 3 sold + 1 restocked)", onHand)
	}
	v = decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", res.ID), nil, ck))
	if v.Status != "partially_returned" || v.Lines[0].ReturnedQty != "2" {
		t.Fatalf("after return: %+v", v)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/return", res.ID), ReturnRequest{Lines: []ReturnLine{{SaleItemID: line, Quantity: "2"}}}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("over-return: %d %s", rec.Code, rec.Body)
	}
}
