package battle

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Random shop days with customers (docs/rules/customers.md): sales on account
// (part cash, over the limit by confirm), change kept to the account, returns of
// their sales, payments by cash / card / cheque / old debt first, advance refunds
// from the till or the safe, bounced cheques, the opening adjusted, the limit
// changed, customers disabled and brought back. The machine keeps each
// customer's balance and old debt by the rule book; after every step the auditor
// runs, each balance and old debt is held to the books (CUST-1, CUST-9, CUST-15),
// each statement ends at its balance (CUST-10) and the drawer at its books (DRAW-1).
func TestBattleCustomers(t *testing.T) {
	t.Parallel()
	setRapid(t)
	rapid.Check(t, func(rt *rapid.T) {
		m := newCustMachine(rt)
		rt.Repeat(map[string]func(*rapid.T){
			"":           m.check,
			"OnAccount":  m.onAccount,
			"KeepChange": m.keepChange,
			"Return":     m.giveBack,
			"Pay":        m.pay,
			"Refund":     m.refund,
			"Bounce":     m.bounce,
			"Opening":    m.opening,
			"Limit":      m.limit,
			"Disable":    m.toggle,
		})
	})
}

type mCust struct {
	id              int64
	bal, old, limit decimal.Decimal
	active          bool
}

// AddBalance's rule: the old debt never passes what they owe (CUST-9, CUST-15).
func (c *mCust) add(d decimal.Decimal) {
	c.bal = c.bal.Add(d)
	c.old = decimal.Min(c.old, decimal.Max(c.bal, decimal.Zero))
}

type mCustSale struct {
	id, line, cust int64
	left           int             // units not yet back (Rs 100 each)
	owed           decimal.Decimal // on account, less what returns cleared
}

type mCheque struct {
	id     int64
	cust   *mCust
	amount decimal.Decimal
}

type custMachine struct {
	s       *shop
	safe    int64
	drawer  decimal.Decimal
	custs   []*mCust
	sales   []*mCustSale
	cheques []*mCheque
	n       int // cheque numbers
}

func newCustMachine(t *rapid.T) *custMachine {
	s := newShop(t)
	m := &custMachine{s: s, safe: s.locker(t, "Safe", "safe", "1000000"), drawer: decimal.NewFromInt(5000)}
	s.open(t, s.till1)
	for i, c := range []struct{ limit, opening string }{{"100000", "0"}, {"500", "0"}, {"5000", "2000"}} {
		id := s.newCustomer(t, fmt.Sprintf("07130000%02d", i), c.limit, c.opening)
		op := dec(t, c.opening)
		m.custs = append(m.custs, &mCust{id: id, bal: op, old: op, limit: dec(t, c.limit), active: true})
	}
	return m
}

func (m *custMachine) cust(t *rapid.T) *mCust { return rapid.SampledFrom(m.custs).Draw(t, "customer") }

// try sends an action the books expect to pass (ok) or be refused.
func (m *custMachine) try(t *rapid.T, ok bool, rule, what string, call func() error) bool {
	err := call()
	if ok && err != nil {
		t.Fatalf("%s %s refused: %v", rule, what, err)
	}
	if !ok && err == nil {
		t.Fatalf("%s %s went through; the books say it can't", rule, what)
	}
	return err == nil
}

func (m *custMachine) onAccount(t *rapid.T) {
	c := m.cust(t)
	qty := rapid.IntRange(1, 5).Draw(t, "qty")
	total := decimal.NewFromInt(int64(100 * qty))
	paid := decimal.NewFromInt(int64(rapid.IntRange(0, qty-1).Draw(t, "cash units") * 100))
	credit := total.Sub(paid)
	confirm := rapid.Bool().Draw(t, "confirm over limit")
	ok := confirm || !credit.GreaterThan(c.limit.Sub(c.bal))
	var r saleResult
	if !m.try(t, ok, "CUST-8", "sale on account "+credit.String(), func() error {
		return m.s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"customerId": c.id, "allowOverLimit": confirm, "items": []line{{"productId": m.s.prod["plain"], "quantity": fmt.Sprint(qty)}},
			"payments": []pay{cash(paid.String()), {"method": "credit", "amount": credit.String()}}}, &r)
	}) {
		return
	}
	c.add(credit)
	m.drawer = m.drawer.Add(paid)
	m.sales = append(m.sales, &mCustSale{id: r.ID, line: m.s.sale(t, m.s.till1, r.ID).Lines[0].ID, cust: c.id, left: qty, owed: credit})
	if !c.active {
		var n int
		m.s.must(t, m.s.db.Get(&n, `SELECT count(*) FROM sale_flags WHERE sale_id = $1 AND kind = 'customer_disabled'`, r.ID))
		if n != 1 {
			t.Fatalf("CUST-17 sale %d to a disabled customer not flagged", r.ID)
		}
	}
}

func (m *custMachine) keepChange(t *rapid.T) {
	c := m.cust(t)
	qty := rapid.IntRange(1, 3).Draw(t, "qty")
	extra := rapid.IntRange(1, 99).Draw(t, "extra")
	kept := rapid.IntRange(1, extra).Draw(t, "kept")
	var r saleResult
	m.s.must(t, m.s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": c.id, "items": []line{{"productId": m.s.prod["plain"], "quantity": fmt.Sprint(qty)}},
		"payments": []pay{cash(fmt.Sprint(100*qty + extra))}, "changeGiven": fmt.Sprint(extra - kept), "onAccountRounding": true}, &r))
	c.add(decimal.NewFromInt(int64(-kept)))
	m.drawer = m.drawer.Add(decimal.NewFromInt(int64(100*qty + kept)))
	m.sales = append(m.sales, &mCustSale{id: r.ID, line: m.s.sale(t, m.s.till1, r.ID).Lines[0].ID, cust: c.id, left: qty})
}

// giveBack returns one unit of a customer's sale: it clears what is still owed on
// that sale, never more than they owe now (RET-18, RET-19); the rest is cash.
func (m *custMachine) giveBack(t *rapid.T) {
	var open []*mCustSale
	for _, s := range m.sales {
		if s.left > 0 {
			open = append(open, s)
		}
	}
	if len(open) == 0 {
		t.Skip("nothing to return")
	}
	s := rapid.SampledFrom(open).Draw(t, "sale")
	var c *mCust
	for _, x := range m.custs {
		if x.id == s.cust {
			c = x
		}
	}
	value := decimal.NewFromInt(100)
	cleared := decimal.Min(value, s.owed, decimal.Max(c.bal, decimal.Zero))
	if !cleared.IsPositive() {
		cleared = decimal.Zero
	}
	if cashBack := m.s.giveBack(t, m.s.till1, s.id, s.line, "1"); !cashBack.Equal(value.Sub(cleared)) {
		t.Fatalf("RET-19 return of sale %d paid %s cash, the books say %s (owed on it %s, balance %s)", s.id, cashBack, value.Sub(cleared), s.owed, c.bal)
	}
	s.left--
	s.owed = s.owed.Sub(cleared)
	c.add(cleared.Neg())
	m.drawer = m.drawer.Sub(value.Sub(cleared))
}

func (m *custMachine) pay(t *rapid.T) {
	c := m.cust(t)
	amt := amount(t, "amount", decimal.Max(c.bal, decimal.Zero).Add(decimal.NewFromInt(500)))
	path := fmt.Sprintf("/api/v1/till/customers/%d/payment", c.id)
	body := map[string]any{"amount": amt.String(), "method": "cash"}
	how := rapid.SampledFrom([]string{"cash", "card", "cheque", "old first"}).Draw(t, "how")
	switch how {
	case "card":
		body["method"], body["accountId"] = "card", m.s.cardAcct
	case "cheque":
		m.n++
		body = map[string]any{"method": "cheque", "cheques": []map[string]string{{"bank": "BOC", "number": fmt.Sprint(900000 + m.n), "amount": amt.String(), "date": "2026-12-01"}}}
	case "old first":
		path = fmt.Sprintf("/api/v1/admin/customers/%d/payment", c.id)
		body["source"], body["oldDebtFirst"] = fmt.Sprintf("locker:%d", m.safe), true
	}
	client := m.s.till1
	if how == "old first" {
		client = m.s.admin
	}
	var r struct{ PaymentID int64 }
	m.s.must(t, client.Do("POST", path, body, &r))
	if how == "old first" {
		p := decimal.Max(decimal.Min(amt, c.old), decimal.Zero)
		c.bal, c.old = c.bal.Sub(p), c.old.Sub(p)
		if rest := amt.Sub(p); rest.IsPositive() {
			c.add(rest.Neg())
		}
	} else {
		c.add(amt.Neg())
	}
	switch how {
	case "cash":
		m.drawer = m.drawer.Add(amt)
	case "cheque":
		var id int64
		m.s.must(t, m.s.db.Get(&id, `SELECT id FROM cheques WHERE customer_payment_id = $1`, r.PaymentID))
		m.cheques = append(m.cheques, &mCheque{id: id, cust: c, amount: amt})
	}
}

// refund hands back part of an advance — from the till (never past the drawer,
// DRAW-3) or the safe; more than the advance is refused (CUST-11).
func (m *custMachine) refund(t *rapid.T) {
	c := m.cust(t)
	adv := c.bal.Neg()
	amt := amount(t, "amount", decimal.Max(adv, decimal.Zero).Add(decimal.NewFromInt(50)))
	fromTill := rapid.Bool().Draw(t, "from the till")
	ok := !amt.GreaterThan(adv) && (!fromTill || !amt.GreaterThan(m.drawer))
	if !m.try(t, ok, "CUST-11", "advance refund "+amt.String(), func() error {
		if fromTill {
			return m.s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", c.id), map[string]any{"amount": amt.String()}, nil)
		}
		return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/refund", c.id), map[string]any{"amount": amt.String(), "source": fmt.Sprintf("locker:%d", m.safe)}, nil)
	}) {
		return
	}
	c.add(amt)
	if fromTill {
		m.drawer = m.drawer.Sub(amt)
	}
}

func (m *custMachine) bounce(t *rapid.T) {
	if len(m.cheques) == 0 {
		t.Skip("no cheque waiting")
	}
	i := rapid.IntRange(0, len(m.cheques)-1).Draw(t, "cheque")
	q := m.cheques[i]
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", q.id), map[string]string{"status": "bounced"}, nil))
	q.cust.add(q.amount)
	m.cheques = append(m.cheques[:i], m.cheques[i+1:]...)
}

// opening sets the old debt: balance moves by the difference; an advance pays it down (CUST-4, CUST-15).
func (m *custMachine) opening(t *rapid.T) {
	c := m.cust(t)
	v := decimal.NewFromInt(int64(rapid.IntRange(-500, 3000).Draw(t, "opening")))
	if !m.try(t, c.active, "CUST-4", "opening "+v.String(), func() error {
		return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", c.id), map[string]string{"opening": v.String()}, nil)
	}) {
		return
	}
	c.bal = c.bal.Add(v.Sub(c.old))
	c.old = v
	if v.IsPositive() {
		c.old = decimal.Min(v, decimal.Max(c.bal, decimal.Zero))
	}
}

func (m *custMachine) limit(t *rapid.T) {
	c := m.cust(t)
	v := decimal.NewFromInt(int64(rapid.IntRange(0, 3000).Draw(t, "limit")))
	if m.try(t, c.active, "CUST-8", "limit "+v.String(), func() error {
		return m.s.till1.Do("PATCH", fmt.Sprintf("/api/v1/till/customers/%d/credit-limit", c.id), map[string]string{"creditLimit": v.String()}, nil)
	}) {
		c.limit = v
	}
}

func (m *custMachine) toggle(t *rapid.T) {
	c := m.cust(t)
	if c.active {
		m.s.must(t, m.s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", c.id), nil, nil))
	} else {
		m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/activate", c.id), nil, nil))
	}
	c.active = !c.active
}

// check: the auditor, then every account, statement and the drawer against the books.
func (m *custMachine) check(t *rapid.T) {
	m.s.audit(t)
	for _, c := range m.custs {
		a := m.s.acct(t, c.id)
		if !a.Balance.Equal(c.bal) || !a.OldDebt.Equal(c.old) {
			t.Fatalf("CUST-1 customer %d owes %s (old debt %s), the books say %s (%s)", c.id, a.Balance, a.OldDebt, c.bal, c.old)
		}
		var st struct {
			Lines []struct{ Balance string }
		}
		m.s.must(t, m.s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/customers/%d/statement", c.id), nil, &st))
		end := decimal.Zero
		if n := len(st.Lines); n > 0 {
			end = dec(t, st.Lines[n-1].Balance)
		}
		if !end.Equal(c.bal) {
			t.Fatalf("CUST-10 customer %d statement ends at %s, the books say %s", c.id, end, c.bal)
		}
	}
	if got := dec(t, m.s.expected(t, m.s.till1)); !got.Equal(m.drawer) {
		t.Fatalf("DRAW-1 drawer expects %s, the books say %s", got, m.drawer)
	}
}
