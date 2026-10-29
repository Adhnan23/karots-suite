package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// An expense whose cheques came back only partly: voiding it returns only the
// money that really left (the cleared cheque), and an expense whose cheque
// bounced can't have its amount changed (Pay again would pay the old unpaid).
func TestExpenseAfterABounce(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	stamp := time.Now().UnixNano()
	var bank, safe int64
	if err := te.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, fmt.Sprintf("T bounce bank %d", stamp)); err != nil {
		t.Fatal(err)
	}
	if err := te.db.Get(&safe, `INSERT INTO lockers (name, kind, allow_negative) VALUES ($1, 'safe', true) RETURNING id`, fmt.Sprintf("T bounce safe %d", stamp)); err != nil {
		t.Fatal(err)
	}
	day := time.Now().Format("2006-01-02")
	line := func(number, amount string) ChequeLine {
		return ChequeLine{LockerID: bank, Number: fmt.Sprintf("%s%d", number, stamp%100000), Amount: amount, Date: day}
	}
	do := func(method, url string, body any, want int) {
		t.Helper()
		if rec := te.do(method, url, body, admin); rec.Code != want {
			t.Fatalf("%s %s: %d %s (want %d)", method, url, rec.Code, rec.Body, want)
		}
	}
	cheque := func(cat, number string) (exp, chq int64) {
		t.Helper()
		if err := te.db.QueryRow(`SELECT e.id, c.id FROM expenses e JOIN cheques c ON c.expense_id = e.id WHERE e.category = $1 AND c.number = $2`,
			cat, fmt.Sprintf("%s%d", number, stamp%100000)).Scan(&exp, &chq); err != nil {
			t.Fatal(err)
		}
		return
	}

	half := fmt.Sprintf("T half bounced %d", stamp)
	do(http.MethodPost, "/api/v1/admin/expenses", AdminExpenseRequest{Category: half, Amount: "1000", Method: "cheque",
		Cheques: []ChequeLine{line("91", "500"), line("92", "500")}}, http.StatusCreated)
	exp, a := cheque(half, "91")
	_, b := cheque(half, "92")
	do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/clear", a), ChequeClearRequest{Date: day}, http.StatusNoContent)
	do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/return", b), ChequeReturnRequest{Status: "bounced"}, http.StatusNoContent)
	do(http.MethodPost, fmt.Sprintf("/api/v1/admin/expenses/%d/void", exp), ExpenseVoidRequest{Dest: fmt.Sprintf("locker:%d", safe)}, http.StatusCreated)
	var back string
	te.db.Get(&back, `SELECT COALESCE(SUM(balance_delta), 0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id = $1`, safe) //nolint:errcheck
	if back != "500.00" {
		t.Fatalf("void after a part bounce put %s back, want 500.00 (only the cleared cheque left)", back)
	}

	whole := fmt.Sprintf("T whole bounced %d", stamp)
	do(http.MethodPost, "/api/v1/admin/expenses", AdminExpenseRequest{Category: whole, Amount: "1000", Method: "cheque",
		Cheques: []ChequeLine{line("93", "1000")}}, http.StatusCreated)
	exp, c := cheque(whole, "93")
	do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/return", c), ChequeReturnRequest{Status: "bounced"}, http.StatusNoContent)
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/expenses/%d", exp), AdminExpenseRequest{Category: whole, Amount: "600"}, admin); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("amount change after a bounce: %d %s", rec.Code, rec.Body)
	}
}
