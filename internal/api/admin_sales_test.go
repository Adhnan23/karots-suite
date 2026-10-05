package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdminSalesAndReturn(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	phone := func() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }
	cashier := te.login(t, phone(), "cashier")
	admin := te.login(t, phone(), "admin")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, cashier)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, cashier)
	var safe int64
	if err := te.db.Get(&safe, `INSERT INTO lockers (name, kind) VALUES ($1, 'safe') RETURNING id`, "T sales safe "+phone()); err != nil {
		t.Fatal(err)
	}
	te.db.Exec(`INSERT INTO locker_ledger (locker_id, balance_delta, kind) VALUES ($1, 1000, 'open_balance')`, safe) //nolint:errcheck
	bal := func() string {
		var s string
		te.db.Get(&s, `SELECT SUM(balance_delta)::numeric(14,2)::text FROM locker_ledger WHERE locker_id=$1`, safe) //nolint:errcheck
		return s
	}

	cashSale := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "3", PaymentRequest{Method: "cash", Amount: "300"}), cashier))
	cust := decode[CustomerCreated](t, te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "T sales cust", Phone: phone(), CreditLimit: "1000"}, cashier)).Customer
	req := sale(f.plainID, "2", PaymentRequest{Method: "credit", Amount: "200"})
	req.CustomerID = &cust.ID
	creditSale := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, cashier))

	// The list finds the sale by receipt; the summary counts every match.
	rows := decode[[]AdminSaleRow](t, te.do(http.MethodGet, "/api/v1/admin/sales?preset=today&q="+cashSale.ReceiptNo, nil, admin))
	if len(rows) != 1 || rows[0].Paid != "300.00" || rows[0].Total != "300.00" {
		t.Fatalf("list %+v", rows)
	}
	sum := decode[AdminSalesSummary](t, te.do(http.MethodGet, "/api/v1/admin/sales/summary?preset=today", nil, admin))
	exp := te.do(http.MethodGet, "/api/v1/admin/sales/export?preset=today&format=csv", nil, admin)
	recs, err := csv.NewReader(strings.NewReader(exp.Body.String())).ReadAll()
	if err != nil || exp.Code != http.StatusOK || len(recs)-1 != sum.Count || sum.Count < 2 {
		t.Fatalf("export %d rows vs summary %d (%v)", len(recs)-1, sum.Count, err)
	}

	// A cash return from a locker: the locker pays the paid share, with a receipt.
	line := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", cashSale.ID), nil, admin)).Lines[0].ID
	ret := func(id int64, src string, qty string, l int64) (int, AdminReturnResult) {
		rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/sales/%d/return", id), AdminReturnRequest{Reason: "T", Source: src,
			Lines: []ReturnLine{{SaleItemID: l, Quantity: qty}}}, admin)
		if rec.Code != http.StatusOK {
			return rec.Code, AdminReturnResult{}
		}
		return rec.Code, decode[AdminReturnResult](t, rec)
	}
	if c, _ := ret(cashSale.ID, "", "1", line); c != http.StatusUnprocessableEntity {
		t.Fatalf("cash refund with no source: %d", c)
	}
	c, r := ret(cashSale.ID, fmt.Sprintf("locker:%d", safe), "1", line)
	if c != http.StatusOK || r.CashRefund != "100.00" || r.ReceiptID == nil || bal() != "900.00" {
		t.Fatalf("locker refund: %d %+v, safe %s", c, r, bal())
	}
	// A credit sale return clears debt and moves no cash (no source needed).
	cl := decode[SaleView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/sales/%d", creditSale.ID), nil, admin)).Lines[0].ID
	if c, r := ret(creditSale.ID, "", "1", cl); c != http.StatusOK || r.CashRefund != "0.00" || r.ReceiptID != nil {
		t.Fatalf("credit return: %d %+v", c, r)
	}
}
