// Package battle is Pocket Cashier's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide).
package battle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pocketcashier"
	_ "karots-suite/plugins/repairs" // a plugin whose writes a phone replays
)

// Pocket Cashier's battle tests (docs/rules/plugins/pocketcashier.md): one
// throwaway shop on the real server with the plugin compiled in, phones that pair
// and send what they did, the auditor (core's and PKT) after each test.

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

func next() int64 { return seq.Add(1) }

// phone is a paired, approved phone sending with its own key.
type phone struct {
	s              *ltest.Shop
	secret, device string
}

// call sends method path as the phone (its key, no sign-in); the status and body.
func (p *phone) call(method, path string, in any) (int, []byte) {
	var body bytes.Buffer
	if in != nil {
		_ = json.NewEncoder(&body).Encode(in)
	}
	req := httptest.NewRequest(method, "/api/v1/x/pocketcashier"+path, &body)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	if p.secret != "" {
		req.Header.Set("X-Ks-Phone", p.secret)
	}
	w := httptest.NewRecorder()
	p.s.H.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// pairOnly asks to pair a new phone; its code (waiting for an admin).
func pairOnly(t *testing.T, s *ltest.Shop) (*phone, string) {
	t.Helper()
	n := next()
	p := &phone{s: s, device: fmt.Sprintf("battle-phone-%d-%d", time.Now().UnixNano(), n)}
	p.secret = fmt.Sprintf("battle-secret-%d-0123456789abcdef0123456789", n)
	code, body := p.call("POST", "/pair", pocketcashier.PairRequest{Name: fmt.Sprintf("Phone %d", n), Device: p.device, Secret: p.secret})
	if code != http.StatusOK {
		t.Fatalf("pair: %d %s", code, body)
	}
	var r pocketcashier.PairReply
	s.Must(t, json.Unmarshal(body, &r))
	return p, r.Code
}

func newPhone(t *testing.T, s *ltest.Shop) *phone {
	t.Helper()
	p, code := pairOnly(t, s)
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/pocketcashier/phones/approve", map[string]string{"code": code}, nil))
	return p
}

func (p *phone) id(t *testing.T) int64 {
	t.Helper()
	var id int64
	p.s.Must(t, p.s.DB.Get(&id, `SELECT id FROM pocket_phones WHERE device = $1`, p.device))
	return id
}

// send posts actions as one batch (nobody signed in on the phone).
func (p *phone) send(t *testing.T, acts ...pocketcashier.Action) pocketcashier.ActionsReply {
	t.Helper()
	return p.sendAs(t, nil, acts...)
}

func (p *phone) sendAs(t *testing.T, signedIn *int64, acts ...pocketcashier.Action) pocketcashier.ActionsReply {
	t.Helper()
	code, body := p.call("POST", "/actions", pocketcashier.ActionsRequest{SignedIn: signedIn, Actions: acts})
	if code != http.StatusOK {
		t.Fatalf("send: %d %s", code, body)
	}
	var r pocketcashier.ActionsReply
	p.s.Must(t, json.Unmarshal(body, &r))
	return r
}

// person makes a staff member (PIN 1234, no PIN change asked) and returns their id.
func person(t *testing.T, s *ltest.Shop, role string) int64 {
	t.Helper()
	n := next()
	var u struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/users", map[string]any{"name": fmt.Sprintf("Phone %s %d", role, n), "phone": fmt.Sprintf("071%07d", n),
		"role": role, "pin": "1234", "canManageCredit": true}, &u))
	s.Must(t, exec(s, `UPDATE users SET must_change_pin = false WHERE id = $1`, u.ID))
	return u.ID
}

func exec(s *ltest.Shop, q string, args ...any) error {
	_, err := s.DB.Exec(q, args...)
	return err
}

func ref() string { return fmt.Sprintf("b%d-%d", time.Now().UnixNano(), next()) }

func act(kind string, uid int64, at time.Time, body any) pocketcashier.Action {
	raw, _ := json.Marshal(body)
	return pocketcashier.Action{Ref: ref(), Kind: kind, UserID: uid, At: at, Body: raw, Handed: true}
}

func open(uid int64, at time.Time, cash string) pocketcashier.Action {
	return act("open", uid, at, pocketcashier.OpenBody{OpeningCash: cash})
}

func closeTill(uid int64, at time.Time, cash string) pocketcashier.Action {
	return act("close", uid, at, pocketcashier.CloseBody{ClosingCash: cash})
}

// cashSale sells qty of product at price, paid in cash exactly.
func cashSale(uid int64, at time.Time, product int64, qty, price, paid string) pocketcashier.Action {
	return act("sale", uid, at, pocketcashier.SaleBody{SaleType: "retail", Total: paid,
		Items:    []pocketcashier.SaleItem{{ProductID: product, Quantity: qty, ExpectPrice: price}},
		Payments: []pocketcashier.SalePayment{{Method: "cash", Amount: paid}}})
}

func call(uid int64, at time.Time, c pocketcashier.CallBody) pocketcashier.Action {
	return act("call", uid, at, c)
}

func mustApplied(t *testing.T, r pocketcashier.ActionsReply, n int) {
	t.Helper()
	if len(r.Results) != n {
		t.Fatalf("want %d results, got %+v", n, r.Results)
	}
	for _, x := range r.Results {
		if x.Status != "applied" {
			t.Fatalf("not applied: %+v", r.Results)
		}
	}
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d).Truncate(time.Millisecond) }

// count is one number from the shop's database.
func count(t *testing.T, s *ltest.Shop, q string, args ...any) int {
	t.Helper()
	var n int
	s.Must(t, s.DB.Get(&n, q, args...))
	return n
}

func str(t *testing.T, s *ltest.Shop, q string, args ...any) string {
	t.Helper()
	var v string
	s.Must(t, s.DB.Get(&v, q, args...))
	return v
}

// PKT-6, PKT-10: a day on the phone — open, sell, close — is an ordinary till day.
func TestPKT6_ADayOnThePhone(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	tea := s.Product(t, fmt.Sprintf("Phone tea %d", next()), "120", "10", nil)
	r := p.send(t, open(uid, ago(time.Hour), "1000"), cashSale(uid, ago(50*time.Minute), tea, "2", "120", "240"),
		closeTill(uid, ago(10*time.Minute), "1240"))
	mustApplied(t, r, 3)
	if got := str(t, s, `SELECT difference::text FROM cash_register WHERE user_id = $1`, uid); got != "0.00" {
		t.Fatalf("PKT-10: over/short %s, want 0.00", got)
	}
	if n := count(t, s, `SELECT count(*) FROM sales WHERE id = $1 AND session_id IS NOT NULL`, r.Results[1].SaleID); n != 1 {
		t.Fatal("PKT-6: the sale isn't in the till")
	}
	s.Audit(t)
}

// PKT-4: a batch sent twice, and two at once, applies once.
func TestPKT4_SentTwiceAppliesOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	tea := s.Product(t, fmt.Sprintf("Phone twice %d", next()), "50", "10", nil)
	acts := []pocketcashier.Action{open(uid, ago(time.Hour), "0"), cashSale(uid, ago(30*time.Minute), tea, "1", "50", "50")}
	first := p.send(t, acts...)
	again := p.send(t, acts...)
	mustApplied(t, first, 2)
	if again.Results[1].SaleID != first.Results[1].SaleID {
		t.Fatalf("PKT-4: %+v then %+v", first.Results, again.Results)
	}
	if n := count(t, s, `SELECT count(*) FROM sales WHERE cashier_id = $1`, uid); n != 1 {
		t.Fatalf("PKT-4: %d sales", n)
	}
	s.Audit(t)
}

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	tea := s.Product(t, fmt.Sprintf("Fire tea %d", next()), "50", "10", nil)
	refused := call(uid, ago(time.Minute), pocketcashier.CallBody{Method: "POST", Path: "/api/v1/till/customers/999999999/payment",
		Body: json.RawMessage(`{"amount":"100","method":"cash"}`), CashIn: "100", Label: "Credit from nobody"})
	r := p.send(t, open(uid, ago(time.Hour), "0"), cashSale(uid, ago(30*time.Minute), tea, "1", "50", "50"), refused)
	if r.Results[2].Status != "failed" || count(t, s, `SELECT count(*) FROM money_receipts WHERE kind = 'refused_on_phone' AND created_by = $1`, uid) != 1 {
		t.Fatalf("PKT-7: the refused call's cash isn't booked: %+v", r.Results)
	}
	s.Audit(t)
	for _, x := range []struct{ rule, bad string }{
		{"PKT-4", fmt.Sprintf(`UPDATE sales SET client_ref = NULL WHERE id = %d`, r.Results[1].SaleID)},
		{"PKT-7", fmt.Sprintf(`UPDATE pocket_actions SET cash_settled = false WHERE ref = '%s'`, refused.Ref)},
		{"PKT-12", fmt.Sprintf(`UPDATE pocket_sends SET actions = actions + 1 WHERE id = (SELECT send_id FROM pocket_actions WHERE ref = '%s')`, refused.Ref)},
		{"PKT-12", fmt.Sprintf(`UPDATE pocket_actions SET status = 'applied' WHERE ref = '%s'`, refused.Ref)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(x.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == x.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", x.rule, x.bad)
		}
	}
}
