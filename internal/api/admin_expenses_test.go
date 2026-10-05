package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/lib/pq"
)

func TestAdminExpenses(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009614", "admin")
	var ids []int64
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM money_receipts WHERE note LIKE '%T ex %'`)               //nolint:errcheck
		te.db.Exec(`DELETE FROM locker_ledger WHERE locker_id = ANY($1)`, pq.Array(ids)) //nolint:errcheck
		te.db.Exec(`DELETE FROM lockers WHERE id = ANY($1)`, pq.Array(ids))              //nolint:errcheck
		te.db.Exec(`DELETE FROM expenses WHERE description LIKE 'T ex %'`)               //nolint:errcheck
	})
	mk := func(name, opening string) int64 {
		l := decode[AdminLocker](t, te.do(http.MethodPost, "/api/v1/admin/lockers", LockerRequest{Name: name, Kind: "safe", OpeningBalance: opening}, ck))
		ids = append(ids, l.ID)
		return l.ID
	}
	bal := func(id int64) string {
		var s string
		if err := te.db.Get(&s, `SELECT COALESCE(SUM(balance_delta),0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return s
	}
	count := func(q string, args ...any) int {
		var n int
		if err := te.db.Get(&n, q, args...); err != nil {
			t.Fatal(err)
		}
		return n
	}
	safe, other := mk("T ex safe", "1000"), mk("T ex other", "0")
	loc := func(id int64) string { return "locker:" + strconv.FormatInt(id, 10) }
	create := func(amount, desc, source string) int {
		return te.do(http.MethodPost, "/api/v1/admin/expenses", AdminExpenseRequest{Category: "Transport", Amount: amount, Description: desc, Source: source}, ck).Code
	}
	find := func(desc string) AdminExpenseRow {
		for _, r := range decode[[]AdminExpenseRow](t, te.do(http.MethodGet, "/api/v1/admin/expenses?preset=today&q=T+ex", nil, ck)) {
			if r.Description == desc {
				return r
			}
		}
		t.Fatalf("expense %q not listed", desc)
		return AdminExpenseRow{}
	}

	// Paid from a locker: debited, its receipt refs the expense, the list says where from.
	rec := te.do(http.MethodPost, "/api/v1/admin/expenses", AdminExpenseRequest{Category: "Transport", Amount: "1,200.50", Description: "T ex taxi", Source: loc(safe)}, ck)
	if rec.Code != http.StatusConflict {
		t.Fatalf("overdraw: %d %s", rec.Code, rec.Body)
	}
	if count(`SELECT count(*) FROM expenses WHERE description='T ex taxi'`) != 0 {
		t.Fatal("an overdraw left the expense behind")
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/expenses", AdminExpenseRequest{Category: "Transport", Amount: "300", Description: "T ex taxi", Source: loc(safe)}, ck)
	if rec.Code != http.StatusCreated || bal(safe) != "700.00" {
		t.Fatalf("create: %d %s, safe %s", rec.Code, rec.Body, bal(safe))
	}
	rid := decode[ReceiptRef](t, rec).ReceiptID
	taxi := find("T ex taxi")
	if rid == nil {
		t.Fatal("no receipt")
	}
	if mr := decode[MoneyReceiptRow](t, te.do(http.MethodGet, "/api/v1/admin/money-receipts/"+strconv.FormatInt(*rid, 10), nil, ck)); mr.RefKind != "expense" || mr.RefID != taxi.ID || mr.Amount != "300.00" {
		t.Fatalf("receipt %+v", mr)
	}
	if taxi.PaidFrom != "T ex safe" || taxi.Amount != "300.00" || count(`SELECT count(*) FROM money_receipts WHERE ref_kind='expense' AND ref_id=$1 AND kind='expense'`, taxi.ID) != 1 {
		t.Fatalf("listed %+v", taxi)
	}
	// Outside the shop: no money leg at all.
	if c := create("50", "T ex online", ""); c != http.StatusCreated {
		t.Fatalf("outside create: %d", c)
	}
	online := find("T ex online")
	if online.PaidFrom != "" {
		t.Fatalf("outside listed %+v", online)
	}

	// The summary totals every match from SQL.
	if s := decode[AdminExpenseSummary](t, te.do(http.MethodGet, "/api/v1/admin/expenses/summary?preset=today&q=T+ex", nil, ck)); s.Total != "350.00" || s.Count != 2 || len(s.Categories) == 0 {
		t.Fatalf("summary %+v", s)
	}

	// Edit: a paid expense keeps its amount; other fields change.
	base := "/api/v1/admin/expenses/" + strconv.FormatInt(taxi.ID, 10)
	if c := te.do(http.MethodPut, base, AdminExpenseRequest{Category: "Transport", Amount: "999", Description: "T ex taxi"}, ck).Code; c != http.StatusUnprocessableEntity {
		t.Fatalf("paid amount edit: %d", c)
	}
	if c := te.do(http.MethodPut, base, AdminExpenseRequest{Category: "Fuel", Amount: "300", Description: "T ex taxi", Date: "2026-10-01"}, ck).Code; c != http.StatusNoContent {
		t.Fatalf("edit: %d", c)
	}
	if c := te.do(http.MethodPut, "/api/v1/admin/expenses/"+strconv.FormatInt(online.ID, 10), AdminExpenseRequest{Category: "Transport", Amount: "55", Description: "T ex online"}, ck).Code; c != http.StatusNoContent {
		t.Fatalf("unpaid amount edit: %d", c)
	}

	// Void: pre-picks the locker it left; a closed till or archived locker is refused.
	info := decode[ExpenseVoidInfo](t, te.do(http.MethodGet, base+"/void", nil, ck))
	if !info.Paid || info.Source != loc(safe) || info.From != "T ex safe" || info.Owner != "" {
		t.Fatalf("void info %+v", info)
	}
	if c := te.do(http.MethodPost, base+"/void", ExpenseVoidRequest{Dest: "till:999999"}, ck).Code; c != http.StatusUnprocessableEntity {
		t.Fatalf("void into a closed till: %d", c)
	}
	te.do(http.MethodDelete, "/api/v1/admin/lockers/"+strconv.FormatInt(other, 10), nil, ck)
	if c := te.do(http.MethodPost, base+"/void", ExpenseVoidRequest{Dest: loc(other)}, ck).Code; c != http.StatusUnprocessableEntity {
		t.Fatalf("void into an archived locker: %d", c)
	}
	if c := te.do(http.MethodPost, base+"/void", ExpenseVoidRequest{}, ck).Code; c != http.StatusUnprocessableEntity {
		t.Fatalf("paid void with nowhere to put it: %d", c)
	}
	rec = te.do(http.MethodPost, base+"/void", ExpenseVoidRequest{Dest: loc(safe)}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID == nil || bal(safe) != "1000.00" {
		t.Fatalf("void: %d %s, safe %s", rec.Code, rec.Body, bal(safe))
	}
	if count(`SELECT count(*) FROM expenses WHERE id=$1`, taxi.ID) != 0 || count(`SELECT count(*) FROM money_receipts WHERE ref_kind='expense' AND ref_id=$1 AND kind='expense_void'`, taxi.ID) != 1 {
		t.Fatal("void left the expense, or wrote no expense_void receipt")
	}
	if c := te.do(http.MethodPost, base+"/void", ExpenseVoidRequest{Dest: loc(safe)}, ck).Code; c != http.StatusNotFound || bal(safe) != "1000.00" {
		t.Fatalf("second void: %d, safe %s", c, bal(safe))
	}
	// An outside-the-shop expense voids with no money leg.
	rec = te.do(http.MethodPost, "/api/v1/admin/expenses/"+strconv.FormatInt(online.ID, 10)+"/void", ExpenseVoidRequest{}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID != nil {
		t.Fatalf("outside void: %d %s", rec.Code, rec.Body)
	}
}
