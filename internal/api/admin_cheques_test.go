package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// The Cheques page and its review actions, end to end: cheques taken from a
// customer and written to a supplier through the money forms, listed and
// summed by the reminder rules, cleared / bounced by a manager (never a
// cashier), and the statements reconcile after a bounce.
func TestAdminChequesReview(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	stamp := time.Now().UnixNano()
	manager := te.login(t, fmt.Sprintf("07%08d", stamp%100_000_000), "manager")
	cashier := te.login(t, fmt.Sprintf("07%08d", stamp%100_000_000+1), "cashier")
	day := func(d int) string { return time.Now().AddDate(0, 0, d).Format("2006-01-02") }
	var bank, cust, sup int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(te.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, fmt.Sprintf("T review bank %d", stamp)))
	must(te.db.Get(&cust, `INSERT INTO customers (name, outstanding_balance) VALUES ($1, 2000) RETURNING id`, fmt.Sprintf("T review cust %d", stamp)))
	must(te.db.Get(&sup, `INSERT INTO suppliers (name, outstanding_balance) VALUES ($1, 5000) RETURNING id`, fmt.Sprintf("T review sup %d", stamp)))
	num := func(n int) string { return fmt.Sprintf("%d%04d", n, stamp%10000) }
	before := decode[ChequeSummary](t, te.do(http.MethodGet, "/api/v1/admin/cheques/summary", nil, manager))

	// Received: one dated today (to deposit), one tomorrow (not yet).
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/customers/%d/payment", cust), CustomerPayRequest{Method: "cheque", Cheques: []ChequeLine{
		{Bank: "Bank of Ceylon", Number: num(1), Amount: "700", Date: day(0)},
		{Bank: "Bank of Ceylon", Number: num(2), Amount: "300", Date: day(1)},
	}}, manager); rec.Code != http.StatusCreated {
		t.Fatalf("customer pays: %d %s", rec.Code, rec.Body)
	}
	// Given on an empty bank: one in 3 days (due), one in 4 (not yet) → bank short.
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), AdminSupplierPayRequest{Amount: "1500", Method: "cheque", Cheques: []ChequeLine{
		{LockerID: bank, Number: num(3), Amount: "1000", Date: day(3)},
		{LockerID: bank, Number: num(4), Amount: "500", Date: day(4)},
	}}, manager); rec.Code != http.StatusCreated {
		t.Fatalf("pay supplier: %d %s", rec.Code, rec.Body)
	}

	after := decode[ChequeSummary](t, te.do(http.MethodGet, "/api/v1/admin/cheques/summary", nil, manager))
	if after.ToDeposit.Count-before.ToDeposit.Count != 1 || after.GivenDue.Count-before.GivenDue.Count != 1 {
		t.Fatalf("reminders: to deposit +%d (want +1), given due +%d (want +1)", after.ToDeposit.Count-before.ToDeposit.Count, after.GivenDue.Count-before.GivenDue.Count)
	}
	short := false
	for _, b := range after.Short {
		short = short || (b.LockerID == bank && b.After == "-1500.00")
	}
	if !short {
		t.Fatalf("bank with pending 1,500 not short: %+v", after.Short)
	}
	locks := decode[AdminLockerList](t, te.do(http.MethodGet, "/api/v1/admin/lockers", nil, manager))
	for _, l := range locks.Items {
		if l.ID == bank && (l.Balance != "0.00" || l.AfterCheques != "-1500.00") {
			t.Fatalf("bank row %+v, want balance 0.00 after cheques -1500.00", l)
		}
	}

	rows := decode[[]ChequeRow](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/cheques?dir=in&customer=%d", cust), nil, manager))
	if len(rows) != 2 || rows[0].Days != 0 || rows[1].Days != 1 || rows[0].Status != "pending" || rows[0].Who == "" {
		t.Fatalf("received list %+v", rows)
	}
	due := decode[[]ChequeRow](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/cheques?dir=out&supplier=%d&status=pending&due=1", sup), nil, manager))
	if len(due) != 1 || due[0].Days != 3 {
		t.Fatalf("given due list %+v", due)
	}

	today, tomorrow := rows[0].ID, rows[1].ID
	// A cashier can't review.
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/clear", today), ChequeClearRequest{LockerID: bank}, cashier); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier clear: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/clear", today), ChequeClearRequest{LockerID: bank}, manager); rec.Code != http.StatusNoContent {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/clear", today), ChequeClearRequest{LockerID: bank}, manager); rec.Code != http.StatusConflict {
		t.Fatalf("clear twice: %d", rec.Code)
	}
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/cheques/%d", tomorrow), ChequeFixRequest{Bank: "Sampath Bank", Number: num(5), Date: day(2)}, manager); rec.Code != http.StatusNoContent {
		t.Fatalf("fix: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/return", tomorrow), ChequeReturnRequest{Status: "bounced"}, manager); rec.Code != http.StatusNoContent {
		t.Fatalf("bounce: %d %s", rec.Code, rec.Body)
	}
	d := decode[ChequeDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/cheques/%d", tomorrow), nil, manager))
	if d.Status != "bounced" || d.Bank != "Sampath Bank" || d.Source.Kind != "customer_payment" || len(d.History) < 2 || d.LastBankID != bank {
		t.Fatalf("detail %+v", d)
	}

	// Statements reconcile: the bounce is a debit line and the balance matches.
	st := decode[Statement](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/customers/%d/statement", cust), nil, manager))
	var owed string
	te.db.Get(&owed, `SELECT outstanding_balance::text FROM customers WHERE id = $1`, cust) //nolint:errcheck
	if st.Balance != owed || !hasLine(st, "Cheque bounced", num(5)+" (Sampath Bank)") {
		t.Fatalf("customer statement balance %s (owed %s), lines %+v", st.Balance, owed, st.Lines)
	}
	given := decode[[]ChequeRow](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/cheques?dir=out&supplier=%d", sup), nil, manager))
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/cheques/%d/return", given[0].ID), ChequeReturnRequest{Status: "cancelled"}, manager); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel given: %d %s", rec.Code, rec.Body)
	}
	sst := decode[Statement](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/suppliers/%d/statement", sup), nil, manager))
	te.db.Get(&owed, `SELECT outstanding_balance::text FROM suppliers WHERE id = $1`, sup) //nolint:errcheck
	if sst.Balance != owed || !hasLine(sst, "Cheque cancelled", "") {
		t.Fatalf("supplier statement balance %s (owed %s), lines %+v", sst.Balance, owed, sst.Lines)
	}

	cf := decode[AdminCashflow](t, te.do(http.MethodGet, "/api/v1/admin/cashflow?preset=today", nil, manager))
	lines := map[string]bool{}
	for _, l := range cf.NetLines {
		lines[l.Label] = true
	}
	if !lines["Cheques in hand"] || !lines["Your cheques not cashed yet"] {
		t.Fatalf("net position lines %+v", cf.NetLines)
	}

	if banks := decode[[]string](t, te.do(http.MethodGet, "/api/v1/cheques/banks", nil, cashier)); len(banks) < 18 {
		t.Fatalf("banks for a cashier: %v", banks)
	}
	if dup := decode[ChequeDuplicate](t, te.do(http.MethodGet, "/api/v1/cheques/duplicate?dir=in&bank=Bank%20of%20Ceylon&number="+num(1), nil, cashier)); !dup.Duplicate {
		t.Fatal("repeated cheque not flagged")
	}
}

func hasLine(st Statement, kind, ref string) bool {
	for _, l := range st.Lines {
		if l.Kind == kind && (ref == "" || l.Ref == ref) {
			return true
		}
	}
	return false
}
