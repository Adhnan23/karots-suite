package api

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestCounterCredit(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)

	var cid int64
	if err := te.db.Get(&cid, `INSERT INTO customers (name, phone, credit_limit, outstanding_balance) VALUES ($1, $2, 5000, 500) RETURNING id`,
		fmt.Sprintf("T owes %d", time.Now().UnixNano()), fmt.Sprintf("075%07d", time.Now().UnixNano()%10_000_000)); err != nil {
		t.Fatal(err)
	}
	pay := func(amount string) CreditPaymentResult {
		t.Helper()
		rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid), CreditPaymentRequest{Amount: amount, Method: "cash"}, ck)
		if rec.Code != http.StatusCreated {
			t.Fatalf("pay %s: %d %s", amount, rec.Code, rec.Body)
		}
		return decode[CreditPaymentResult](t, rec)
	}
	expected := func() string {
		return decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck)).Summary.Expected
	}

	if r := pay("200"); r.BalanceAfter != "300.00" || r.ReceiptNo == "" || r.PaymentID == 0 {
		t.Fatalf("first payment: %+v", r)
	}
	if expected() != "1200.00" {
		t.Fatalf("drawer after collecting 200: %s", expected())
	}
	if r := pay("400"); r.BalanceAfter != "-100.00" {
		t.Fatalf("overpayment should leave a 100 advance: %+v", r)
	}
	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/refund", cid), AdvanceRefundRequest{Amount: "100"}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID == nil {
		t.Fatalf("refund advance: %d %s", rec.Code, rec.Body)
	}
	if expected() != "1500.00" { // 1000 + 200 + 400 − 100
		t.Fatalf("drawer after refund: %s", expected())
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/refund", cid), AdvanceRefundRequest{Amount: "1"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("refund with no advance left: %d", rec.Code)
	}

	// Receipts hub: both collections are listed with their balances; the drawer
	// moves (open, collections, refund) show as CR- money receipts.
	var name string
	te.db.Get(&name, `SELECT name FROM customers WHERE id=$1`, cid) //nolint:errcheck
	debts := decode[[]DebtReceiptRow](t, te.do(http.MethodGet, "/api/v1/till/customer-payments?q="+url.QueryEscape(name), nil, ck))
	if len(debts) != 2 || debts[0].Method != "Cash" || debts[0].BalanceAfter == "" {
		t.Fatalf("credit receipts: %+v", debts)
	}
	money := decode[[]MoneyReceiptRow](t, te.do(http.MethodGet, "/api/v1/till/money-receipts?q="+url.QueryEscape(name), nil, ck))
	if len(money) < 3 || money[0].KindLabel == "" {
		t.Fatalf("money receipts for %s: %+v", name, money)
	}
}
