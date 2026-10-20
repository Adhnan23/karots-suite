package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// A cashier went home with the till open: a manager closes it for them. The money is
// booked exactly as the cashier's own close; closed_by and Activity say who did it.
func TestTillCloseForThem(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	ck := te.login(t, phone, "cashier")
	mgr := te.login(t, fmt.Sprintf("07%08d", (time.Now().UnixNano()+7)%100_000_000), "manager")
	var cashierID, mgrID int64
	te.db.Get(&cashierID, `SELECT id FROM users WHERE phone = $1`, phone)                     //nolint:errcheck
	te.db.Get(&mgrID, `SELECT id FROM users WHERE role = 'manager' ORDER BY id DESC LIMIT 1`) //nolint:errcheck

	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 3}}, ck)
	st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	id := st.Summary.Session.ID
	if st.Summary.Stale {
		t.Fatal("a till opened just now isn't stale")
	}

	// Serving (the cashier's cart changed just now): refused — never cut a sale off.
	te.beat("till-pc", PresenceBeat{Path: "/till", CartItems: 1, CartTotal: "50.00"}, ck)
	url := fmt.Sprintf("/api/v1/admin/tills/%d/close", id)
	if rec := te.do(http.MethodPost, url, CloseDrawerRequest{Breakdown: map[string]float64{"500": 2}}, mgr); errCode(rec) != "TILL_SERVING" {
		t.Fatalf("close while serving: %d %s", rec.Code, rec.Body)
	}
	te.beat("till-pc", PresenceBeat{Path: "/till"}, ck) // cart empty again

	rec := te.do(http.MethodPost, url, CloseDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 2}}, mgr)
	res := decode[CloseResult](t, rec)
	if rec.Code != 200 || res.Counted != "1200.00" || res.ExpectedCash != "1300.00" || res.Difference != "-100.00" {
		t.Fatalf("close for them %d %s", rec.Code, rec.Body)
	}
	// Only once: a second close (theirs or the cashier's) is refused, nothing booked twice.
	if rec := te.do(http.MethodPost, url, CloseDrawerRequest{Breakdown: map[string]float64{"500": 2}}, mgr); rec.Code != 409 {
		t.Fatalf("second close for them %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{Breakdown: map[string]float64{}}, ck); rec.Code != 409 {
		t.Fatalf("cashier close after %d %s", rec.Code, rec.Body)
	}
	var closings int
	var moveUser int64
	te.db.Get(&closings, `SELECT COUNT(*) FROM cash_movements WHERE session_id = $1 AND type = 'closing'`, id) //nolint:errcheck
	te.db.Get(&moveUser, `SELECT user_id FROM cash_movements WHERE session_id = $1 AND type = 'closing'`, id)  //nolint:errcheck
	if closings != 1 || moveUser != cashierID {
		t.Fatalf("closing rows %d by %d (want 1 by the cashier %d)", closings, moveUser, cashierID)
	}
	var logged int
	te.db.Get(&logged, `SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND detail LIKE '%till for them%'`, mgrID) //nolint:errcheck
	if logged != 1 {
		t.Fatalf("audit rows by the manager: %d", logged)
	}
	v := decode[TillSessionView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/tills/%d", id), nil, mgr))
	if v.Session.Open || v.Session.ClosedBy == "" || v.Session.ClosedLate {
		t.Fatalf("session view %+v", v.Session)
	}
	// A cashier can't reach it.
	if rec := te.do(http.MethodPost, url, CloseDrawerRequest{}, ck); rec.Code != 403 {
		t.Fatalf("cashier close for them %d", rec.Code)
	}
}

// A till opened yesterday (and long enough ago) is stale; the till then makes the cashier close it.
func TestTillStale(t *testing.T) {
	te := newTestEnv(t)
	phone := fmt.Sprintf("07%08d", (time.Now().UnixNano()+13)%100_000_000)
	ck := te.login(t, phone, "cashier")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"100": 1}}, ck)
	te.db.Exec(`UPDATE cash_register SET opened_at = now() - interval '30 hours' WHERE user_id = (SELECT id FROM users WHERE phone = $1) AND closed_at IS NULL`, phone) //nolint:errcheck
	st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if !st.Summary.Stale {
		t.Fatalf("30 h old till should be stale: %+v", st.Summary)
	}
}
