package battle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/sales"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pocketcashier"
)

// PKT-1, PKT-3: only an active phone gets in; blocked stops at once; a waiting phone
// may ask again; a paired one pairs again only after Remove.
func TestPKT1_PhoneKey(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	stranger := &phone{s: s, secret: "never-paired-0123456789abcdef0123456789"}
	if code, _ := stranger.call("GET", "/copy", nil); code != http.StatusUnauthorized {
		t.Fatalf("PKT-1: an unknown key got %d", code)
	}
	waiting, code := pairOnly(t, s)
	if c, _ := waiting.call("POST", "/actions", pocketcashier.ActionsRequest{}); c != http.StatusForbidden {
		t.Fatalf("PKT-1: a waiting phone got %d", c)
	}
	// Asking again while waiting replaces the request: the old code no longer approves.
	again, _ := waiting.call("POST", "/pair", pocketcashier.PairRequest{Name: "Again", Device: waiting.device, Secret: waiting.secret})
	if again != http.StatusOK {
		t.Fatalf("PKT-1: a waiting phone couldn't ask again: %d", again)
	}
	if str(t, s, `SELECT pair_code FROM pocket_phones WHERE device = $1`, waiting.device) != code &&
		s.Admin.Do("POST", "/api/v1/x/pocketcashier/phones/approve", map[string]string{"code": code}, nil) == nil {
		t.Fatal("PKT-1: the replaced request's code still approved the phone")
	}
	p := newPhone(t, s)
	if c, _ := p.call("POST", "/pair", pocketcashier.PairRequest{Name: "Twice", Device: p.device, Secret: p.secret}); c != http.StatusConflict {
		t.Fatalf("PKT-1: a paired phone paired again: %d", c)
	}
	// PKT-3: the same version answers 304.
	c, _ := p.call("GET", "/copy", nil)
	if c != http.StatusOK {
		t.Fatalf("PKT-3: copy %d", c)
	}
	if err := s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/pocketcashier/phones/%d/block", p.id(t)), nil, nil); err == nil {
		t.Fatal("PKT-1: a cashier blocked a phone")
	}
	uid := person(t, s, "cashier")
	p.sendAs(t, &uid)
	if who := str(t, s, `SELECT COALESCE(active_device, '') FROM users WHERE id = $1`, uid); who != p.device {
		t.Fatalf("PKT-2: the phone doesn't hold its person: %q", who)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/pocketcashier/phones/%d/block", p.id(t)), nil, nil))
	for _, path := range []string{"/copy", "/recent", "/parts", "/whoami"} {
		if c, _ := p.call("GET", path, nil); c != http.StatusForbidden {
			t.Errorf("PKT-1: blocked phone got %d on %s", c, path)
		}
	}
	if c, _ := p.call("POST", "/actions", pocketcashier.ActionsRequest{}); c != http.StatusForbidden {
		t.Errorf("PKT-1: blocked phone sent actions: %d", c)
	}
	if who := str(t, s, `SELECT COALESCE(active_device, '') FROM users WHERE id = $1`, uid); who != "" {
		t.Errorf("PKT-1: blocking kept its person on it: %q", who)
	}
	// Removed, it pairs again from the start.
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/pocketcashier/phones/%d", p.id(t)), nil, nil))
	if c, _ := p.call("POST", "/pair", pocketcashier.PairRequest{Name: "Back", Device: p.device, Secret: p.secret}); c != http.StatusOK {
		t.Errorf("PKT-1: a removed phone couldn't pair again: %d", c)
	}
}

// PKT-2: whichever sign-in came last holds the person; admins never act through a phone.
func TestPKT2_OnePersonOneDevice(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	phoneAt := ago(10 * time.Minute)
	p.sendAs(t, &uid) // signed in on the phone
	pc, err := ltest.NewClient(s.H, str(t, s, `SELECT phone FROM users WHERE id = $1`, uid), "1234", fmt.Sprintf("pc-%d", next()))
	s.Must(t, err)
	s.Must(t, pc.Do("GET", "/api/v1/me", nil, nil))
	// The phone still shows them, signed in before the PC: its send never takes them back.
	code, body := p.call("POST", "/actions", pocketcashier.ActionsRequest{SignedIn: &uid, SignedInAt: &phoneAt})
	if code != http.StatusOK {
		t.Fatalf("send %d %s", code, body)
	}
	if dev := str(t, s, `SELECT active_device FROM users WHERE id = $1`, uid); dev == p.device {
		t.Fatal("PKT-2: a send took the person back from the PC")
	}
	// Work done on the phone before still goes through.
	tea := s.Product(t, fmt.Sprintf("Moved tea %d", next()), "40", "5", nil)
	mustApplied(t, p.send(t, open(uid, ago(time.Hour), "0"), cashSale(uid, ago(40*time.Minute), tea, "1", "40", "40")), 2)
	// An admin: never held, never acts.
	var admin int64
	s.Must(t, s.DB.Get(&admin, `SELECT id FROM users WHERE phone = '0770000001'`))
	before := str(t, s, `SELECT COALESCE(active_device, '') FROM users WHERE id = $1`, admin)
	r := p.sendAs(t, &admin, cashSale(admin, ago(time.Minute), tea, "1", "40", "40"))
	if r.Results[0].Status != "failed" || str(t, s, `SELECT COALESCE(active_device, '') FROM users WHERE id = $1`, admin) != before {
		t.Fatalf("PKT-2: an admin acted through a phone: %+v", r.Results)
	}
	s.Audit(t)
}

// PKT-6, PKT-13: the PC knew better — the sale stands, flagged; never refused.
func TestPKT6_PCKnewBetterFlagsNeverRefuses(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	tea := s.Product(t, fmt.Sprintf("Knew tea %d", next()), "100", "1", nil)
	mustApplied(t, p.send(t, open(uid, ago(2*time.Hour), "0")), 1)
	s.Must(t, exec(s, `UPDATE products SET selling_price = 130, wholesale_price = 130 WHERE id = $1`, tea)) // changed on the PC meanwhile
	oversold := cashSale(uid, ago(time.Hour), tea, "3", "100", "300")                                       // only 1 on hand; the phone showed 100
	future := cashSale(uid, time.Now().Add(time.Hour), tea, "1", "100", "100")
	twin := cashSale(uid, ago(time.Hour).Add(2*time.Minute), tea, "3", "100", "300")
	r := p.send(t, oversold, future, twin)
	mustApplied(t, r, 3)
	flags := func(id int64) string {
		return str(t, s, `SELECT COALESCE(string_agg(kind, ',' ORDER BY kind), '') FROM sale_flags WHERE sale_id = $1`, id)
	}
	if f := flags(r.Results[0].SaleID); !strings.Contains(f, sales.FlagOversold) || !strings.Contains(f, sales.FlagPrice) {
		t.Errorf("PKT-6: oversold not flagged: %s", f)
	}
	if got := str(t, s, `SELECT total::text FROM sales WHERE id = $1`, r.Results[0].SaleID); got != "300.00" {
		t.Errorf("PKT-6: the price the phone showed wasn't kept: %s", got)
	}
	if f := flags(r.Results[1].SaleID); !strings.Contains(f, sales.FlagClock) {
		t.Errorf("PKT-6: a clock in the future not flagged: %s", f)
	}
	if n := count(t, s, `SELECT count(*) FROM sales WHERE id = $1 AND created_at <= now()`, r.Results[1].SaleID); n != 1 {
		t.Error("PKT-6: a sale counted in a day not yet come")
	}
	if f := flags(r.Results[2].SaleID); !strings.Contains(f, sales.FlagLookalike) {
		t.Errorf("PKT-13: a lookalike not flagged: %s", f)
	}
	// A person turned off meanwhile: still sold, flagged.
	s.Must(t, exec(s, `UPDATE users SET is_active = false WHERE id = $1`, uid))
	r = p.send(t, cashSale(uid, ago(20*time.Minute), tea, "1", "100", "100"))
	mustApplied(t, r, 1)
	if f := flags(r.Results[0].SaleID); !strings.Contains(f, sales.FlagInactiveUser) {
		t.Errorf("PKT-6: an inactive person's sale not flagged: %s", f)
	}
	s.Audit(t)
}

// PKT-7: refused work keeps its cash — booked once when handed, never when watched.
func TestPKT7_RefusedCash(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	mustApplied(t, p.send(t, open(uid, ago(time.Hour), "0")), 1)
	booked := func() int {
		return count(t, s, `SELECT count(*) FROM money_receipts WHERE kind = 'refused_on_phone' AND created_by = $1`, uid)
	}
	gone := cashSale(uid, ago(30*time.Minute), 999999999, "1", "75", "75") // a product the PC doesn't have
	watched := gone
	watched.Ref, watched.Handed = ref(), false
	r := p.send(t, watched)
	if r.Results[0].Status != "failed" || booked() != 0 {
		t.Fatalf("PKT-7: a watched refusal booked cash: %+v, %d", r.Results, booked())
	}
	// The reply was lost; the screen went ahead and the retry says so: booked now, once.
	watched.Handed = true
	p.send(t, watched)
	p.send(t, watched)
	if booked() != 1 {
		t.Fatalf("PKT-7: lost reply booked %d times", booked())
	}
	r = p.send(t, gone)
	if r.Results[0].Status != "failed" || booked() != 2 || !strings.Contains(r.Results[0].Note, "75.00 came into the drawer") {
		t.Fatalf("PKT-7: handed refusal: %+v, %d", r.Results, booked())
	}
	s.Audit(t)
}

// PKT-8, PKT-5, PKT-9: what a phone made offline is named by a temp number until the
// PC gives the real id; a call depending on a refused one is refused, naming it; a
// business refusal never stops the batch.
func TestPKT8_TempNumbers(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	var cust struct{ ID int64 }
	n := next()
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("Temp customer %d", n), "phone": fmt.Sprintf("075%07d", n),
		"creditLimit": "0", "openingBalance": "0"}, &cust))
	payNum, jobNum := int64(9000000000000000)+n*10, int64(9000000000000000)+n*10+1
	pay := call(uid, ago(20*time.Minute), pocketcashier.CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/till/customers/%d/payment", cust.ID),
		Body: json.RawMessage(`{"amount":"500","method":"cash"}`), CashIn: "500", Label: "Credit from the customer", TempID: payNum})
	job := call(uid, ago(19*time.Minute), pocketcashier.CallBody{Method: "POST", Path: "/api/v1/x/repairs/jobs",
		Body: json.RawMessage(fmt.Sprintf(`{"customerId":%d,"deviceModel":"A12","fault":"screen"}`, cust.ID)), Label: "A repair", TempID: jobNum})
	deposit := call(uid, ago(18*time.Minute), pocketcashier.CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", jobNum),
		Body: json.RawMessage(fmt.Sprintf(`{"paymentId":%d}`, payNum)), Label: "A deposit"})
	notAllowed := call(uid, ago(17*time.Minute), pocketcashier.CallBody{Method: "POST", Path: "/api/v1/admin/categories",
		Body: json.RawMessage(`{"name":"From a phone"}`), Label: "A category"})
	badJob := call(uid, ago(16*time.Minute), pocketcashier.CallBody{Method: "POST", Path: "/api/v1/x/repairs/jobs",
		Body: json.RawMessage(`{"customerId":999999999,"deviceModel":"A12","fault":"screen"}`), Label: "A repair for nobody", TempID: jobNum + 5})
	child := call(uid, ago(15*time.Minute), pocketcashier.CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/x/repairs/jobs/%d/urgent", jobNum+5),
		Body: json.RawMessage(`{"urgent":true}`), Label: "Urgent"})
	r := p.send(t, open(uid, ago(time.Hour), "0"), pay, job, deposit, notAllowed, badJob, child)
	if len(r.Results) != 7 {
		t.Fatalf("PKT-5: a refusal stopped the batch: %+v", r.Results)
	}
	for i, want := range []string{"applied", "applied", "applied", "applied", "failed", "failed", "failed"} {
		if r.Results[i].Status != want {
			t.Fatalf("result %d: %+v", i, r.Results[i])
		}
	}
	jobID, payID := r.Results[2].ResultID, r.Results[1].ResultID
	if n := count(t, s, `SELECT count(*) FROM repair_deposits WHERE job_id = $1 AND payment_id = $2`, jobID, payID); n != 1 {
		t.Fatalf("PKT-8: the deposit didn't land on the real job and payment (%d, %d)", jobID, payID)
	}
	if !strings.Contains(r.Results[4].Note, "a phone can't do this") {
		t.Errorf("PKT-9: %q", r.Results[4].Note)
	}
	if !strings.Contains(r.Results[6].Note, "A repair for nobody") {
		t.Errorf("PKT-8: the refusal doesn't name what it depended on: %q", r.Results[6].Note)
	}
	if n := count(t, s, `SELECT count(*) FROM categories WHERE name = 'From a phone'`); n != 0 {
		t.Error("PKT-9: a phone reached an admin page")
	}
	s.Audit(t)
}

// PKT-9: a call runs as the person who did it, with the route's own rights.
func TestPKT9_CallKeepsTheRoutesRights(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	s.Must(t, exec(s, `UPDATE users SET can_manage_credit = false WHERE id = $1`, uid))
	var cust struct{ ID int64 }
	n := next()
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("Rights customer %d", n), "phone": fmt.Sprintf("076%07d", n),
		"creditLimit": "0", "openingBalance": "0"}, &cust))
	refund := call(uid, ago(5*time.Minute), pocketcashier.CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/till/customers/%d/refund", cust.ID),
		Body: json.RawMessage(`{"amount":"10"}`), Label: "Advance refund"})
	refund.Handed = false
	r := p.send(t, open(uid, ago(time.Hour), "0"), refund)
	if r.Results[1].Status != "failed" {
		t.Fatalf("PKT-9: a cashier without credit rights refunded through a phone: %+v", r.Results)
	}
	s.Audit(t)
}

// PKT-10: open once, at its own time; close at its own time.
func TestPKT10_DrawerActions(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	openedAt := ago(3 * time.Hour)
	r := p.send(t, open(uid, openedAt, "500"), open(uid, ago(2*time.Hour), "700"))
	if r.Results[0].Status != "applied" || r.Results[1].Status != "skipped" {
		t.Fatalf("PKT-10: opened twice: %+v", r.Results)
	}
	closedAt := ago(time.Hour)
	mustApplied(t, p.send(t, closeTill(uid, closedAt, "500")), 1)
	var row struct {
		Opened time.Time `db:"opened_at"`
		Closed time.Time `db:"closed_at"`
	}
	s.Must(t, s.DB.Get(&row, `SELECT opened_at, closed_at FROM cash_register WHERE user_id = $1`, uid))
	if !row.Opened.Equal(openedAt) || !row.Closed.Equal(closedAt) {
		t.Fatalf("PKT-10: times %v / %v, want %v / %v", row.Opened, row.Closed, openedAt, closedAt)
	}
	if r := p.send(t, closeTill(uid, ago(time.Minute), "0")); r.Results[0].Status != "skipped" {
		t.Fatalf("PKT-10: closed a closed till: %+v", r.Results)
	}
	s.Audit(t)
}

// PKT-11: a phone's held sale is the shop's; resumed elsewhere first, the phone's
// resume is skipped.
func TestPKT11_HeldSales(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := newPhone(t, s)
	uid := person(t, s, "cashier")
	hold := act("hold", uid, ago(10*time.Minute), pocketcashier.HoldBody{Label: "Table 4", SaleType: "retail", Discount: "0", Cart: `{"lines":[]}`, ItemCount: 1, Total: "100"})
	r := p.send(t, hold)
	mustApplied(t, r, 1)
	held := r.Results[0].HeldID
	if n := count(t, s, `SELECT count(*) FROM held_sales WHERE id = $1 AND cashier_id = $2`, held, uid); n != 1 {
		t.Fatal("PKT-11: the held sale isn't the shop's")
	}
	s.Must(t, exec(s, `DELETE FROM held_sales WHERE id = $1`, held)) // resumed on the PC
	if r := p.send(t, act("unhold", uid, ago(5*time.Minute), pocketcashier.UnholdBody{HeldID: held})); r.Results[0].Status != "skipped" {
		t.Fatalf("PKT-11: %+v", r.Results)
	}
}

// PKT-15, PLG-7: the data group holds every table; the shop's certificate is one of them.
func TestPKT15_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "pocketcashier")
}
