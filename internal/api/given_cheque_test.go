package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Paying a supplier and an expense by cheque from admin and the counter:
// nothing leaves the bank now, the cheques are pending and linked; an expense
// whose cheque is pending can't be voided or have its amount changed; once the
// cheque bounces the expense is unpaid and Pay settles it as the same expense.
func TestGivenChequesThroughTheAPI(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	stamp := time.Now().UnixNano()
	var bank, sup int64
	if err := te.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, fmt.Sprintf("T given bank %d", stamp)); err != nil {
		t.Fatal(err)
	}
	if err := te.db.Get(&sup, `INSERT INTO suppliers (name, outstanding_balance) VALUES ($1, 3000) RETURNING id`, fmt.Sprintf("T given supplier %d", stamp)); err != nil {
		t.Fatal(err)
	}
	bankMoved := func() (n int) {
		te.db.Get(&n, `SELECT count(*) FROM locker_ledger WHERE locker_id = $1`, bank) //nolint:errcheck
		return n
	}
	line := func(number, amount string) ChequeLine {
		return ChequeLine{LockerID: bank, Number: fmt.Sprintf("%s%d", number, stamp%10000), Amount: amount, Date: time.Now().Format("2006-01-02")}
	}
	post := func(url string, body any) {
		t.Helper()
		if rec := te.do(http.MethodPost, url, body, admin); rec.Code != http.StatusCreated {
			t.Fatalf("POST %s: %d %s", url, rec.Code, rec.Body)
		}
	}

	post(fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), AdminSupplierPayRequest{Amount: "1000", Method: "cheque", Cheques: []ChequeLine{line("71", "600"), line("72", "400")}})
	post(fmt.Sprintf("/api/v1/till/suppliers/%d/pay", sup), SupplierPayRequest{Amount: "500", Method: "cheque", Cheques: []ChequeLine{line("73", "500")}})
	var owed string
	te.db.Get(&owed, `SELECT outstanding_balance::text FROM suppliers WHERE id = $1`, sup) //nolint:errcheck
	if owed != "1500.00" {
		t.Fatalf("supplier owed %s, want 1500.00", owed)
	}
	var n int
	te.db.Get(&n, `SELECT count(*) FROM cheques WHERE supplier_id = $1 AND status = 'pending' AND supplier_payment_id IS NOT NULL`, sup) //nolint:errcheck
	if n != 3 || bankMoved() != 0 {
		t.Fatalf("pending supplier cheques %d (want 3), bank rows %d (want 0)", n, bankMoved())
	}
	// More cheques than the payment is refused.
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup),
		AdminSupplierPayRequest{Amount: "100", Method: "cheque", Cheques: []ChequeLine{line("74", "150")}}, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("cheques over the payment: %d %s", rec.Code, rec.Body)
	}

	cat := fmt.Sprintf("T cheque expense %d", stamp)
	post("/api/v1/admin/expenses", AdminExpenseRequest{Category: cat, Amount: "2500", Method: "cheque", Cheques: []ChequeLine{line("81", "2500")}})
	post("/api/v1/till/expenses", ExpenseRequest{Category: cat, Amount: "300", Method: "cheque", Cheques: []ChequeLine{line("82", "300")}})
	var exp, chq int64
	te.db.QueryRow(`SELECT e.id, c.id FROM expenses e JOIN cheques c ON c.expense_id = e.id WHERE e.category = $1 AND e.amount = 2500`, cat).Scan(&exp, &chq) //nolint:errcheck
	if exp == 0 || bankMoved() != 0 {
		t.Fatalf("expense by cheque: expense %d, bank rows %d", exp, bankMoved())
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/expenses/%d/void", exp), ExpenseVoidRequest{}, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("void with a pending cheque: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/expenses/%d", exp), AdminExpenseRequest{Category: cat, Amount: "2600"}, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("amount change with a pending cheque: %d %s", rec.Code, rec.Body)
	}

	te.db.Exec(`UPDATE cheques SET status = 'bounced', status_date = CURRENT_DATE WHERE id = $1`, chq) //nolint:errcheck
	te.db.Exec(`UPDATE expenses SET unpaid = amount WHERE id = $1`, exp)                               //nolint:errcheck
	post(fmt.Sprintf("/api/v1/admin/expenses/%d/pay", exp), AdminExpensePayRequest{Method: "cheque", Cheques: []ChequeLine{line("83", "2500")}})
	var unpaid string
	te.db.Get(&unpaid, `SELECT unpaid::text FROM expenses WHERE id = $1`, exp) //nolint:errcheck
	te.db.Get(&n, `SELECT count(*) FROM cheques WHERE expense_id = $1`, exp)   //nolint:errcheck
	if unpaid != "0.00" || n != 2 {
		t.Fatalf("after pay again: unpaid %s (want 0.00), cheques %d (want 2)", unpaid, n)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/expenses/%d/pay", exp), AdminExpensePayRequest{Method: "cash"}, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("paying a paid expense: %d %s", rec.Code, rec.Body)
	}
}
