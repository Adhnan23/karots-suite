package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// A plugin tender this test binary registers: it records what it was asked to settle.
var repaid []plugin.TenderSettle

func init() {
	r := new(plugin.Registry)
	r.AddTenderMethod(plugin.TenderMethod{Value: "fixture_pay", Label: "Fixture pay",
		SettleTx: func(_ context.Context, _ *sqlx.Tx, ev plugin.TenderSettle) error {
			repaid = append(repaid, ev)
			return nil
		}})
	r.AddTenderMethod(plugin.TenderMethod{Value: "fixture_held", Label: "Held", Hidden: true,
		SettleTx: func(context.Context, *sqlx.Tx, plugin.TenderSettle) error { return nil }})
}

// A customer's repayment can go by a plugin tender (till and admin): the payment
// row and the plugin's booking commit together, and the drawer doesn't move.
// A hidden tender is never offered for it.
func TestCreditRepaymentByPluginTender(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	admin := te.login(t, fmt.Sprintf("07%08d", (time.Now().UnixNano()+7)%100_000_000), "admin")
	var cid int64
	if err := te.db.Get(&cid, `INSERT INTO customers (name, phone, credit_limit, outstanding_balance) VALUES ($1, $2, 5000, 500) RETURNING id`,
		fmt.Sprintf("T repays %d", time.Now().UnixNano()), fmt.Sprintf("076%07d", time.Now().UnixNano()%10_000_000)); err != nil {
		t.Fatal(err)
	}
	repaid = nil
	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid),
		CreditPaymentRequest{Amount: "200", Method: "fixture_pay", Meta: map[string]string{"device": "3"}}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("till repayment: %d %s", rec.Code, rec.Body)
	}
	r := decode[CreditPaymentResult](t, rec)
	if r.BalanceAfter != "300.00" || len(repaid) != 1 || repaid[0].CustomerPaymentID != r.PaymentID ||
		repaid[0].CustomerID != cid || repaid[0].Meta["device"] != "3" || repaid[0].Amount.String() != "200" {
		t.Fatalf("till repayment booked %+v (result %+v)", repaid, r)
	}
	rec = te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/customers/%d/payment", cid), CustomerPayRequest{Amount: "100", Method: "fixture_pay"}, admin)
	if rec.Code != http.StatusCreated || len(repaid) != 2 || decode[CreditPaymentResult](t, rec).BalanceAfter != "200.00" {
		t.Fatalf("admin repayment: %d %s (%d settled)", rec.Code, rec.Body, len(repaid))
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid), CreditPaymentRequest{Amount: "10", Method: "fixture_held"}, ck); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("a hidden tender took a repayment: %d %s", rec.Code, rec.Body)
	}
	ms := decode[[]TenderMethod](t, te.do(http.MethodGet, "/api/v1/admin/tender-methods", nil, admin))
	if len(ms) != 1 || ms[0].Value != "fixture_pay" {
		t.Fatalf("admin tender methods = %+v", ms)
	}
}
