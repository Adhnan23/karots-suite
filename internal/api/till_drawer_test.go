package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestTillDrawerLifecycle(t *testing.T) {
	te := newTestEnv(t)
	// A fresh phone each run: opening/moving the drawer anchors this user via
	// RESTRICT FKs (cash_register.user_id, cash_movements.user_id), so seedUser's
	// cleanup can never delete it and a fixed phone would collide on the next run.
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	ck := te.login(t, phone, "cashier")

	st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session != nil || len(st.Denominations) == 0 || st.Lockers == nil {
		t.Fatalf("fresh user: %+v", st)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 3}}, ck)
	if rec.Code != 201 {
		t.Fatalf("open %d %s", rec.Code, rec.Body)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session == nil || st.Summary.Session.OpeningCash != "1300.00" || st.Summary.Expected != "1300.00" {
		t.Fatalf("after open %+v", st.Summary)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/deposit", MoneyMoveRequest{Amount: "200", Reason: "change float"}, ck); rec.Code != 200 {
		t.Fatalf("deposit %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/withdraw", MoneyMoveRequest{Amount: "99999"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("over-withdraw must be refused, got %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/withdraw", MoneyMoveRequest{Amount: "100", Reason: "bank"}, ck); rec.Code != 200 {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Expected != "1400.00" || st.Summary.PayIns != "200.00" || st.Summary.Withdrawals != "100.00" {
		t.Fatalf("after moves %+v", st.Summary)
	}
	// Logout is refused while the till is open (M0 behaviour the SPA routes on).
	if rec := te.do(http.MethodPost, "/api/v1/auth/logout", nil, ck); errCode(rec) != "TILL_OPEN" {
		t.Fatalf("logout with open till: %d %s", rec.Code, rec.Body)
	}
	rec = te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 3}}, ck)
	res := decode[CloseResult](t, rec)
	if rec.Code != 200 || res.Counted != "1300.00" || res.ExpectedCash != "1400.00" || res.Difference != "-100.00" {
		t.Fatalf("close %d %+v", rec.Code, res)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session != nil || st.Summary.LastClosing == nil || st.Summary.LastBreakdown["500"] != 2 {
		t.Fatalf("after close %+v", st.Summary)
	}
	// Nothing counted (the drawer was emptied) closes at zero, not a validation error.
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"100": 1}}, ck)
	rec = te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{Breakdown: map[string]float64{}}, ck)
	if res := decode[CloseResult](t, rec); rec.Code != 200 || res.Counted != "0.00" || res.Difference != "-100.00" {
		t.Fatalf("empty close %d %s", rec.Code, rec.Body)
	}
}

func TestTillNoSaleNeedsDrawerEnabled(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009302", "cashier")
	te.db.Exec(`UPDATE settings SET open_cash_drawer = false`) //nolint:errcheck
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/no-sale", nil, ck); rec.Code != 403 {
		t.Fatalf("drawer disabled: %d", rec.Code)
	}
}
