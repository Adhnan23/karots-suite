package battle

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pocketcashier"
)

// Cases from gremlins' survivors (Task 7): each guards a path no other case reached.

// PKT-7: a cash out refused for a locker that's gone still left the drawer — booked out, not in.
func TestPKT7_RefusedCashOutLeavesTheDrawer(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := opened(t, s, p)
	r := p.send(t, act("withdraw", uid, ago(time.Minute), pocketcashier.MoveBody{Amount: "100", Reason: "bank", LockerID: 999999999}))
	if r.Results[0].Status != "failed" || !strings.Contains(r.Results[0].Note, "100 left the drawer") {
		t.Fatalf("PKT-7: %+v", r.Results)
	}
	if n := count(t, s, `SELECT count(*) FROM cash_movements WHERE user_id = $1 AND amount = -100`, uid); n != 1 {
		t.Errorf("PKT-7: the refused cash out isn't booked out of the drawer (%d)", n)
	}
	s.Audit(t)
}

// PKT-10: an open whose own time can't be used (the phone's clock ahead) opens now, noted.
func TestPKT10_OpenAtAnUnusableTime(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	mustApplied(t, p.send(t, open(uid, time.Now().Add(time.Hour), "200")), 1)
	if n := count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1 AND closed_at IS NULL AND opened_at <= now()
		AND review_note LIKE '%counted from when it arrived%'`, uid); n != 1 {
		t.Errorf("PKT-10: the open wasn't made now and noted (%d)", n)
	}
	s.Audit(t)
}

// PKT-7, PKT-9: cash work after the till was closed goes into a till opened for it — taken
// by the route, or refused by it.
func TestPKT9_CallCashAfterTheTillClosed(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	for _, x := range []struct {
		name, path string
		applied    bool
	}{{"taken", "", true}, {"refused", "/api/v1/till/customers/999999999/payment", false}} {
		uid := person(t, s, "cashier")
		path := x.path
		if path == "" {
			var c struct{ ID int64 }
			n := next()
			s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("Late customer %d", n),
				"phone": fmt.Sprintf("076%07d", n), "creditLimit": "0", "openingBalance": "0"}, &c))
			path = fmt.Sprintf("/api/v1/till/customers/%d/payment", c.ID)
		}
		r := p.send(t, call(uid, ago(time.Minute), pocketcashier.CallBody{Method: "POST", Path: path,
			Body: json.RawMessage(`{"amount":"150","method":"cash"}`), CashIn: "150", Label: "Credit"}))
		if got := r.Results[0].Status == "applied"; got != x.applied {
			t.Fatalf("%s: %+v", x.name, r.Results)
		}
		if n := count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1 AND review_note LIKE 'opened for late work%'`, uid); n != 1 {
			t.Errorf("PKT-7 %s: no till opened for the cash (%d)", x.name, n)
		}
		if !x.applied && !strings.Contains(r.Results[0].Note, "is booked as a cash move") {
			t.Errorf("PKT-7 %s: the refused cash isn't booked: %q", x.name, r.Results[0].Note)
		}
	}
	s.Audit(t)
}

// PKT-9: a call that moves no cash never opens a till for it.
func TestPKT9_CallWithNoCashOpensNoTill(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	var c struct{ ID int64 }
	n := next()
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("No-cash customer %d", n),
		"phone": fmt.Sprintf("077%07d", n), "creditLimit": "0", "openingBalance": "0"}, &c))
	mustApplied(t, p.send(t, call(uid, ago(time.Minute), pocketcashier.CallBody{Method: "POST", Path: "/api/v1/x/repairs/jobs",
		Body: json.RawMessage(fmt.Sprintf(`{"customerId":%d,"deviceModel":"A12","fault":"screen"}`, c.ID)), Label: "A repair"})), 1)
	if n := count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1`, uid); n != 0 {
		t.Errorf("PKT-9: a call with no cash opened %d tills", n)
	}
}
