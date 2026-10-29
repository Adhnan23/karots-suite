package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/features/customers"

	"github.com/shopspring/decimal"
)

// A customer owing 1,200 pays with two cheques (1,000 + 500), at the counter and
// in admin: they end with a 300 advance, the two cheques are pending and linked
// to one customer payment, their money is in Cheques in hand and none reached a
// till or a bank.
func TestCustomerPaysByCheques(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	cashier := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, cashier)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, cashier)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000+1), "admin")

	two := []ChequeLine{
		{Bank: "Bank of Ceylon", Number: "100001", Amount: "1000", Date: "2026-10-29"},
		{Bank: "Sampath Bank", Number: "200002", Amount: "500", Date: "2026-11-15"},
	}
	cases := []struct {
		name string
		ck   *http.Cookie
		url  string
		body any
	}{
		{"counter", cashier, "/api/v1/till/customers/%d/payment", CreditPaymentRequest{Method: "cheque", Cheques: two}},
		{"admin", admin, "/api/v1/admin/customers/%d/payment", CustomerPayRequest{Method: "cheque", Cheques: two}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cid int64
			if err := te.db.Get(&cid, `INSERT INTO customers (name, credit_limit, outstanding_balance) VALUES ($1, 5000, 1200) RETURNING id`,
				fmt.Sprintf("T cheque payer %d", time.Now().UnixNano())); err != nil {
				t.Fatal(err)
			}
			inHand := func() (d decimal.Decimal) {
				te.db.Get(&d, `SELECT COALESCE(SUM(balance_delta),0) FROM locker_ledger WHERE locker_id = (SELECT id FROM lockers WHERE kind = 'cheque')`) //nolint:errcheck
				return d
			}
			before := inHand()
			rec := te.do(http.MethodPost, fmt.Sprintf(tc.url, cid), tc.body, tc.ck)
			if rec.Code != http.StatusCreated {
				t.Fatalf("pay: %d %s", rec.Code, rec.Body)
			}
			r := decode[CreditPaymentResult](t, rec)
			if r.BalanceAfter != "-300.00" || r.PaymentID == 0 {
				t.Fatalf("result %+v, want a 300 advance", r)
			}
			var n int
			te.db.Get(&n, `SELECT count(*) FROM cheques WHERE customer_id = $1 AND customer_payment_id = $2 AND status = 'pending' AND direction = 'in'`, cid, r.PaymentID) //nolint:errcheck
			if n != 2 {
				t.Fatalf("pending cheques linked to the payment = %d, want 2", n)
			}
			var method, amount string
			te.db.QueryRow(`SELECT method, amount::text FROM customer_payments WHERE id = $1`, r.PaymentID).Scan(&method, &amount) //nolint:errcheck
			if method != "cheque" || amount != "1500.00" {
				t.Fatalf("payment row %s %s, want cheque 1500.00", method, amount)
			}
			if got := inHand().Sub(before); !got.Equal(decimal.NewFromInt(1500)) {
				t.Fatalf("Cheques in hand moved %s, want 1500", got)
			}
			var other int
			te.db.Get(&other, `SELECT count(*) FROM locker_ledger WHERE ref_kind = 'customer_payment' AND ref_id = $1
				AND locker_id <> (SELECT id FROM lockers WHERE kind = 'cheque')`, r.PaymentID) //nolint:errcheck
			if other != 0 {
				t.Fatalf("cheque money reached %d other places", other)
			}
			slip, err := customers.NewService(te.db).GetPayment(context.Background(), r.PaymentID)
			if err != nil || len(slip.Cheques) != 2 || slip.Cheques[0].Number != "100001" || slip.Cheques[1].Bank != "Sampath Bank" {
				t.Fatalf("debt slip cheques: %v %+v", err, slip)
			}
		})
	}

	// A cheque line without a number is refused.
	var cid int64
	te.db.Get(&cid, `INSERT INTO customers (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T bad cheque %d", time.Now().UnixNano())) //nolint:errcheck
	rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid),
		CreditPaymentRequest{Method: "cheque", Cheques: []ChequeLine{{Bank: "Bank of Ceylon", Amount: "100", Date: "2026-10-29"}}}, cashier)
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("cheque without a number: %d %s", rec.Code, rec.Body)
	}
}
