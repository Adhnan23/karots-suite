package pocketcashier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// rfix: a cashier, a product at 120 with stock, an active phone, untracked
// cash allowed (all restored afterwards).
type rfix struct {
	en       *env
	cashier  int64
	product  int64
	secret   string
	device   string
	refStamp string
}

func newRfix(t *testing.T, stock string) *rfix {
	en := newEnv(t)
	var was bool
	en.db.Get(&was, `SELECT allow_untracked_cash FROM settings WHERE id = 1`) //nolint:errcheck
	en.db.MustExec(`UPDATE settings SET allow_untracked_cash = true WHERE id = 1`)
	t.Cleanup(func() { en.db.Exec(`UPDATE settings SET allow_untracked_cash = $1 WHERE id = 1`, was) }) //nolint:errcheck
	f := &rfix{en: en, refStamp: fmt.Sprint(time.Now().UnixNano())}
	f.cashier, _ = en.user("cashier")
	en.db.MustExec(`UPDATE users SET name = $2 WHERE id = $1`, f.cashier, "T cashier "+f.refStamp)
	if err := en.db.Get(&f.product, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 50, 120) RETURNING id`, "T tea "+f.refStamp); err != nil {
		t.Fatal(err)
	}
	en.db.MustExec(`UPDATE stock SET quantity = $2 WHERE product_id = $1`, f.product, stock)
	f.secret, f.device = en.activePhone()
	t.Cleanup(func() {
		uid := f.cashier
		for _, q := range []string{
			`DELETE FROM pocket_actions WHERE user_id = $1`,
			`DELETE FROM sales WHERE cashier_id = $1`,
			`DELETE FROM cash_movements WHERE user_id = $1`,
			`DELETE FROM locker_ledger WHERE counter_till_session IN (SELECT id FROM cash_register WHERE user_id = $1)`,
			`DELETE FROM cash_register WHERE user_id = $1`,
			`DELETE FROM held_sales WHERE cashier_id = $1`,
		} {
			en.db.Exec(q, uid) //nolint:errcheck
		}
		en.db.Exec(`DELETE FROM stock_movements WHERE product_id = $1`, f.product) //nolint:errcheck
		en.db.Exec(`DELETE FROM stock_batches WHERE product_id = $1`, f.product)   //nolint:errcheck
		en.db.Exec(`DELETE FROM products WHERE id = $1`, f.product)                //nolint:errcheck
	})
	return f
}

func (f *rfix) ref(s string) string { return f.refStamp + "-" + s }

func ago(d time.Duration) time.Time { return time.Now().Add(-d).Truncate(time.Second) }

func (f *rfix) saleAct(ref string, at time.Time, qty, paid string) Action {
	body, _ := json.Marshal(SaleBody{SaleType: "retail", Items: []SaleItem{{ProductID: f.product, Quantity: qty, ExpectPrice: "120"}},
		Payments: []SalePayment{{Method: "cash", Amount: paid}}})
	return Action{Ref: f.ref(ref), Kind: "sale", UserID: f.cashier, At: at, Body: body}
}

func act(ref, kind string, user int64, at time.Time, body any) Action {
	b, _ := json.Marshal(body)
	return Action{Ref: ref, Kind: kind, UserID: user, At: at, Body: b}
}

func (f *rfix) send(signedIn *int64, acts ...Action) ActionsReply {
	f.en.t.Helper()
	rec := f.en.do(http.MethodPost, "/actions", ActionsRequest{SignedIn: signedIn, Actions: acts}, nil, f.secret)
	if rec.Code != http.StatusOK {
		f.en.t.Fatalf("actions %d %s", rec.Code, rec.Body)
	}
	return decodeAs[ActionsReply](f.en.t, rec)
}

func (f *rfix) count(q string, args ...any) int {
	var n int
	if err := f.en.db.Get(&n, q, args...); err != nil {
		f.en.t.Fatal(err)
	}
	return n
}

func TestReplayRefOnce(t *testing.T) {
	f := newRfix(t, "10")
	f.en.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash, opened_at) VALUES ($1, 0, now() - interval '1 hour')`, f.cashier)
	batch := []Action{f.saleAct("a", ago(30*time.Minute), "1", "120")}
	first := f.send(nil, batch...)
	second := f.send(nil, batch...)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) || first.Results[0].Status != "applied" || first.Results[0].SaleID == 0 {
		t.Fatalf("first %s second %s", a, b)
	}
	if n := f.count(`SELECT count(*) FROM sales WHERE cashier_id = $1`, f.cashier); n != 1 {
		t.Fatalf("%d sales", n)
	}
}

func TestReplayConcurrentSendsApplyOnce(t *testing.T) {
	f := newRfix(t, "10")
	f.en.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash, opened_at) VALUES ($1, 0, now() - interval '1 hour')`, f.cashier)
	batch := []Action{f.saleAct("a", ago(30*time.Minute), "1", "120"), f.saleAct("b", ago(29*time.Minute), "1", "120"), f.saleAct("c", ago(28*time.Minute), "1", "120")}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); f.send(nil, batch...) }()
	}
	wg.Wait()
	if n := f.count(`SELECT count(*) FROM sales WHERE cashier_id = $1`, f.cashier); n != 3 {
		t.Fatalf("%d sales, want 3", n)
	}
}

func TestReplayOrder(t *testing.T) {
	f := newRfix(t, "10")
	opened, closed := ago(50*time.Minute), ago(10*time.Minute)
	r := f.send(nil,
		act(f.ref("open"), "open", f.cashier, opened, OpenBody{OpeningCash: "1000"}),
		f.saleAct("s1", ago(40*time.Minute), "1", "120"),
		f.saleAct("s2", ago(30*time.Minute), "2", "300"),
		act(f.ref("close"), "close", f.cashier, closed, CloseBody{ClosingCash: "1360"}),
	)
	for _, x := range r.Results {
		if x.Status != "applied" {
			t.Fatalf("%+v", r.Results)
		}
	}
	var s struct {
		Opened   time.Time       `db:"opened_at"`
		Closed   time.Time       `db:"closed_at"`
		Expected decimal.Decimal `db:"expected_cash"`
	}
	if err := f.en.db.Get(&s, `SELECT opened_at, closed_at, expected_cash FROM cash_register WHERE user_id = $1`, f.cashier); err != nil {
		t.Fatal(err)
	}
	if !s.Opened.Equal(opened) || !s.Closed.Equal(closed) || !s.Expected.Equal(decimal.NewFromInt(1360)) {
		t.Fatalf("session %+v", s)
	}
	if n := f.count(`SELECT count(*) FROM sale_flags WHERE sale_id IN (SELECT id FROM sales WHERE cashier_id = $1)`, f.cashier); n != 0 {
		t.Fatalf("%d flags", n)
	}
}

func TestReplayMissingProductFailsButBatchGoesOn(t *testing.T) {
	f := newRfix(t, "10")
	f.en.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash, opened_at) VALUES ($1, 0, now() - interval '1 hour')`, f.cashier)
	gone := f.saleAct("gone", ago(20*time.Minute), "1", "120")
	gone.Body, _ = json.Marshal(SaleBody{SaleType: "retail", Items: []SaleItem{{ProductID: 999_999_999, Quantity: "1"}},
		Payments: []SalePayment{{Method: "cash", Amount: "120"}}})
	r := f.send(nil, gone, f.saleAct("ok", ago(19*time.Minute), "1", "120"))
	if r.Results[0].Status != "failed" || r.Results[0].Note == "" || r.Results[1].Status != "applied" {
		t.Fatalf("%+v", r.Results)
	}
	var failed int
	f.en.db.Get(&failed, `SELECT failed FROM pocket_sends s JOIN pocket_actions a ON a.send_id = s.id WHERE a.ref = $1`, gone.Ref) //nolint:errcheck
	if failed != 1 {
		t.Fatalf("send failed = %d", failed)
	}
	// Acknowledged: a resend gives the same failure back, nothing new.
	if again := f.send(nil, gone); again.Results[0].Status != "failed" {
		t.Fatalf("%+v", again.Results)
	}
}
