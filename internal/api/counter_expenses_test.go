package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCounterExpenses(t *testing.T) {
	te := newTestEnv(t)
	phone := func() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }

	// A cashier without the supplier flag never reaches the tab.
	cashier := te.login(t, phone(), "cashier")
	if rec := te.do(http.MethodGet, "/api/v1/till/expenses", nil, cashier); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier without flag: %d", rec.Code)
	}

	ck := te.login(t, phone(), "manager")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	if rec := te.do(http.MethodPost, "/api/v1/till/expenses", ExpenseRequest{Category: "Transport", Amount: "150", Description: "tuk"}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("record: %d %s", rec.Code, rec.Body)
	} else if decode[ReceiptRef](t, rec).ReceiptID == nil {
		t.Fatal("no slip for a cash expense")
	}
	if st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck)); st.Summary.Expected != "850.00" {
		t.Fatalf("drawer after expense: %+v", st.Summary)
	}
	pg := decode[ExpensesPage](t, te.do(http.MethodGet, "/api/v1/till/expenses", nil, ck))
	if len(pg.Today) != 1 || pg.Today[0].Amount != "150.00" || pg.Today[0].Description != "tuk" || len(pg.Categories) == 0 {
		t.Fatalf("page: %+v", pg)
	}
	// A locker the cashier can't use is refused; a blank category is a validation error.
	if rec := te.do(http.MethodPost, "/api/v1/till/expenses", ExpenseRequest{Category: "X", Amount: "1", CounterMoneyRequest: CounterMoneyRequest{LockerID: 999999}}, ck); rec.Code != http.StatusForbidden {
		t.Fatalf("foreign locker: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/expenses", ExpenseRequest{Amount: "1"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("blank category: %d", rec.Code)
	}
	te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
}
