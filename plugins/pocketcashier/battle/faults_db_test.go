package battle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pocketcashier"
)

// kindCase is one kind of action for the fault sweep: setup makes what it needs
// (unfailed) and the action; did counts what it did in the shop.
type kindCase struct {
	name  string
	setup func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action
	did   func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int
}

// refusedKind: the PC refuses it, and its handed cash is booked, once (PKT-7).
const refusedKind = "refused sale"

func opened(t *testing.T, s *ltest.Shop, p *phone) int64 {
	uid := person(t, s, "cashier")
	mustApplied(t, p.send(t, open(uid, ago(time.Hour), "1000")), 1)
	return uid
}

var sweepKinds = []kindCase{
	{"open", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return open(person(t, s, "cashier"), ago(time.Hour), "500")
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1`, a.UserID)
	}},
	{"close", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return closeTill(opened(t, s, p), ago(time.Minute), "1000")
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1 AND closed_at IS NOT NULL`, a.UserID)
	}},
	{"withdraw", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return act("withdraw", opened(t, s, p), ago(time.Minute), pocketcashier.MoveBody{Amount: "100", Reason: "bank"})
	}, moves("withdrawal")},
	{"deposit", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return act("deposit", opened(t, s, p), ago(time.Minute), pocketcashier.MoveBody{Amount: "100", Reason: "float"})
	}, moves("pay_in")},
	{"hold", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return act("hold", person(t, s, "cashier"), ago(time.Minute), pocketcashier.HoldBody{Label: "Later", SaleType: "retail", Discount: "0", Cart: `{"lines":[]}`, ItemCount: 1, Total: "10"})
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM held_sales WHERE cashier_id = $1`, a.UserID)
	}},
	{"unhold", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		uid := person(t, s, "cashier")
		h := act("hold", uid, ago(2*time.Minute), pocketcashier.HoldBody{Label: "Later", SaleType: "retail", Discount: "0", Cart: `{"lines":[]}`, ItemCount: 1, Total: "10"})
		r := p.send(t, h)
		mustApplied(t, r, 1)
		return act("unhold", uid, ago(time.Minute), pocketcashier.UnholdBody{HeldID: r.Results[0].HeldID})
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return 1 - count(t, s, `SELECT count(*) FROM held_sales WHERE cashier_id = $1`, a.UserID)
	}},
	{"sale", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		uid := opened(t, s, p)
		return cashSale(uid, ago(time.Minute), s.Product(t, fmt.Sprintf("Sweep tea %d", next()), "60", "5", nil), "1", "60", "60")
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM sales WHERE cashier_id = $1`, a.UserID)
	}},
	{refusedKind, func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		return cashSale(opened(t, s, p), ago(time.Minute), 999999999, "1", "75", "75") // a product the PC doesn't have
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM money_receipts WHERE kind = 'refused_on_phone' AND created_by = $1`, a.UserID)
	}},
	{"call", func(t *testing.T, s *ltest.Shop, p *phone) pocketcashier.Action {
		uid := opened(t, s, p)
		var c struct{ ID int64 }
		n := next()
		s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("Sweep customer %d", n), "phone": fmt.Sprintf("074%07d", n),
			"creditLimit": "0", "openingBalance": "0"}, &c))
		return call(uid, ago(time.Minute), pocketcashier.CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/till/customers/%d/payment", c.ID),
			Body: json.RawMessage(`{"amount":"200","method":"cash"}`), CashIn: "200", Label: "Credit"})
	}, func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		var c pocketcashier.CallBody
		s.Must(t, json.Unmarshal(a.Body, &c))
		var id int64
		fmt.Sscanf(strings.TrimPrefix(c.Path, "/api/v1/till/customers/"), "%d", &id) //nolint:errcheck
		return count(t, s, `SELECT count(*) FROM customer_payments WHERE customer_id = $1`, id)
	}},
}

func moves(kind string) func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
	return func(t *testing.T, s *ltest.Shop, a pocketcashier.Action) int {
		return count(t, s, `SELECT count(*) FROM cash_movements WHERE user_id = $1 AND type = $2::cash_movement_type`, a.UserID, kind)
	}
}

// PKT-4, PKT-5, PLG-4: a database failure at any statement of any kind of action
// never refuses it for good and never does it twice. The phone sends again (as it
// does, every 5 s) until it has an answer: then it was done once — or, when the
// failure came after the work and before its record, it is never run again and
// says "may not have gone through" (user, 2026-11-21).
func TestPKT5_FailureSweep(t *testing.T) { // plants failures in the one shop: runs alone
	fs := ltest.Shared(t)
	s := fs.Shop
	p := newPhone(t, s)
	for _, kc := range sweepKinds {
		for k := 1; ; k++ {
			if k > 300 {
				t.Fatalf("%s: still failing after 300 statements", kc.name)
			}
			a := kc.setup(t, s, p)
			fs.Fault.Arm(k)
			code, body := p.call("POST", "/actions", pocketcashier.ActionsRequest{Actions: []pocketcashier.Action{a}})
			fired := fs.Fault.Fired()
			failed := fs.Fault.Failed()
			if !fired {
				if code != http.StatusOK {
					t.Fatalf("%s: %d %s with nothing planted", kc.name, code, body)
				}
				t.Logf("%s: %d statements", kc.name, k-1)
				break
			}
			var r pocketcashier.ActionsReply
			if code == http.StatusOK {
				s.Must(t, json.Unmarshal(body, &r))
			}
			if len(r.Results) == 1 && r.Results[0].Status == "failed" && (kc.name != refusedKind || strings.Contains(r.Results[0].Note, "couldn't be booked")) {
				t.Errorf("PKT-5 %s: refused for the PC's own trouble at statement %d (%.80s): %s", kc.name, k, failed, r.Results[0].Note)
				continue
			}
			// The phone tries again until it has an answer.
			var final pocketcashier.ActionResult
			for try := 0; try < 30 && final.Status == ""; try++ {
				if rr := p.send(t, a); len(rr.Results) == 1 {
					final = rr.Results[0]
				}
			}
			did := kc.did(t, s, a)
			switch {
			case final.Status == "failed" && strings.Contains(final.Note, "may not have gone through"):
				if did > 1 {
					t.Errorf("PKT-4 %s: done %d times (statement %d)", kc.name, did, k)
				}
			case kc.name == refusedKind && final.Status == "failed":
				if did != 1 {
					t.Errorf("PKT-7 %s: its cash booked %d times (statement %d, %.80s)", kc.name, did, k, failed)
				}
			case final.Status == "applied" || final.Status == "skipped":
				if did != 1 {
					t.Errorf("PKT-4 %s: %s but done %d times (statement %d, %.80s)", kc.name, final.Status, did, k, failed)
				}
			default:
				t.Errorf("PKT-5 %s: after a failure at statement %d (%.80s): %+v", kc.name, k, failed, final)
			}
			ltest.Check(t, t.Context(), fs.Look)
		}
	}
}

// DRAW-19, PKT-10: a cash out / in made on the phone already happened — booked as
// made, past what the PC counts in the drawer, with untracked cash turned off since,
// and after its till was closed (in a till opened for it); the till is flagged.
func TestDRAW19_PhoneCashBookedAsMade(t *testing.T) { // turns a shop setting off: runs alone
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := opened(t, s, p) // Rs 1,000 in the drawer
	s.Must(t, exec(s, `UPDATE settings SET allow_untracked_cash = false WHERE id = 1`))
	t.Cleanup(func() { _ = exec(s, `UPDATE settings SET allow_untracked_cash = true WHERE id = 1`) })
	out := act("withdraw", uid, ago(30*time.Minute), pocketcashier.MoveBody{Amount: "5000", Reason: "paid the supplier"})
	shut := closeTill(uid, ago(20*time.Minute), "800")
	in := act("deposit", uid, ago(10*time.Minute), pocketcashier.MoveBody{Amount: "300", Reason: "change"})
	r := p.send(t, out, shut, in)
	mustApplied(t, r, 3)
	if n := count(t, s, `SELECT count(*) FROM cash_movements WHERE user_id = $1 AND type = 'withdrawal' AND amount = -5000`, uid); n != 1 {
		t.Errorf("DRAW-19: the cash out wasn't booked as made (%d)", n)
	}
	if n := count(t, s, `SELECT count(*) FROM cash_movements WHERE user_id = $1 AND type = 'pay_in' AND amount = 300`, uid); n != 1 {
		t.Errorf("DRAW-19: the cash in after the close wasn't booked (%d)", n)
	}
	if n := count(t, s, `SELECT count(*) FROM cash_register WHERE user_id = $1 AND COALESCE(review_note, '') <> ''`, uid); n != 2 {
		t.Errorf("DRAW-19: want both tills flagged, %d are", n)
	}
	s.Audit(t)
}

// PKT-4: every kind of action, sent two at once and then again, is done once and
// every send gets the same answer.
func TestPKT4_EveryKindAtOnceAndTwice(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	for _, kc := range sweepKinds {
		p := newPhone(t, s)
		a := kc.setup(t, s, p)
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for i := range codes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], _ = p.call("POST", "/actions", pocketcashier.ActionsRequest{Actions: []pocketcashier.Action{a}})
			}()
		}
		wg.Wait()
		if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
			t.Fatalf("%s: sends answered %v", kc.name, codes)
		}
		r := p.send(t, a)
		if len(r.Results) != 1 {
			t.Fatalf("PKT-4 %s: %+v", kc.name, r.Results)
		}
		want := r.Results[0].Status == "applied" || r.Results[0].Status == "skipped"
		if kc.name == refusedKind {
			want = r.Results[0].Status == "failed" // refused, its cash booked once (did, below)
		}
		if !want {
			t.Errorf("PKT-4 %s: %+v", kc.name, r.Results)
		}
		if did := kc.did(t, s, a); did != 1 {
			t.Errorf("PKT-4 %s: done %d times", kc.name, did)
		}
	}
	s.Audit(t)
}
