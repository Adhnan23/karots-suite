package battle

import (
	"flag"
	"fmt"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Random shop days for selling, paying and returning: every action through the
// real API, then the auditor and the machine's own books (drawer cash, the
// customer's balance, what each sale refunded) after every step. A failure
// shrinks to the fewest steps that break it.
func TestBattleSales(t *testing.T) {
	t.Parallel()
	setRapid(t)
	rapid.Check(t, func(rt *rapid.T) {
		m := &machine{s: newShop(rt), drawer: decimal.NewFromInt(5000)}
		m.s.open(rt, m.s.till1)
		rt.Repeat(map[string]func(*rapid.T){
			"":            m.check,
			"Sell":        m.sell,
			"Return":      m.giveBack,
			"CustomerPay": m.customerPays,
			"Bounce":      m.bounce,
			"CloseOpen":   m.closeOpen,
		})
	})
}

// setRapid: make test runs a few short days; make battle (KS_BATTLE_STEPS) many.
func setRapid(t *testing.T) {
	checks, steps := 3, 40
	if n := steps0(); n > 200 {
		checks, steps = n/100, 100
	}
	for k, v := range map[string]int{"rapid.checks": checks, "rapid.steps": steps} {
		if f := flag.Lookup(k); f != nil && f.Value.String() == f.DefValue {
			f.Value.Set(strconv.Itoa(v)) //nolint:errcheck
		}
	}
}

func steps0() int { return steps() }

type mLine struct {
	id          int64
	qty, left   decimal.Decimal
	group, sets int
}

type mSale struct {
	id                  int64
	total, paid, credit decimal.Decimal // paid = money taken; credit = put on account
	cleared, refunded   decimal.Decimal // debt cleared by returns; everything refunded
	customer            bool
	lines               []mLine
	cheque              int64 // a cheque still pending, 0 = none
	chequeAmt           decimal.Decimal
}

type machine struct {
	s       *shop
	drawer  decimal.Decimal // the till's expected cash
	balance decimal.Decimal // the customer's account
	sales   []*mSale
}

var kinds = []string{"plain", "odd", "cheap", "loose", "pack"}

func (m *machine) sell(t *rapid.T) {
	var items []item
	n := rapid.IntRange(1, 5).Draw(t, "lines")
	for i := 0; i < n; i++ {
		it := item{kind: rapid.SampledFrom(kinds).Draw(t, "kind")}
		if it.kind == "loose" {
			it.qty = decimal.New(int64(rapid.IntRange(1, 5000).Draw(t, "grams")), -3)
		} else {
			it.qty = decimal.NewFromInt(int64(rapid.IntRange(1, 30).Draw(t, "qty")))
		}
		switch rapid.IntRange(0, 2).Draw(t, "discType") {
		case 1:
			it.discType, it.disc = "fixed", decimal.New(int64(rapid.IntRange(0, int(unitPrice[it.kind].Shift(2).IntPart())+50).Draw(t, "discFixed")), -2)
		case 2:
			it.discType, it.disc = "percent", decimal.New(int64(rapid.IntRange(0, 1000).Draw(t, "discPct")), -1)
		}
		items = append(items, it)
	}
	if rapid.IntRange(0, 4).Draw(t, "combo") == 0 { // a combo of two whole-unit lines
		sets := rapid.IntRange(1, 3).Draw(t, "sets")
		for _, k := range []string{"plain", "odd"} {
			items = append(items, item{kind: k, qty: decimal.NewFromInt(int64(sets * rapid.IntRange(1, 2).Draw(t, "perSet"))), group: 1, sets: sets})
		}
	}
	billType, bill := "fixed", decimal.Zero
	switch rapid.IntRange(0, 2).Draw(t, "bill") {
	case 1:
		bill = decimal.New(int64(rapid.IntRange(0, 50000).Draw(t, "billFixed")), -2)
	case 2:
		billType, bill = "percent", decimal.New(int64(rapid.IntRange(0, 1000).Draw(t, "billPct")), -1)
	}
	want := total(items, billType, bill)

	body := map[string]any{"saleType": "retail", "discount": bill.String(), "discountType": billType}
	var lines []line
	for _, it := range items {
		l := line{"productId": m.s.prod[it.kind], "quantity": it.qty.String(), "allowOversell": true}
		if it.discType != "" {
			l["discount"], l["discountType"] = it.disc.String(), it.discType
		}
		if it.group > 0 {
			l["group"] = map[string]any{"no": it.group, "label": "Combo", "sets": strconv.Itoa(it.sets)}
		}
		lines = append(lines, l)
	}
	body["items"] = lines

	sale := &mSale{total: want}
	cashIn := decimal.Zero
	how := rapid.SampledFrom([]string{"cash", "cash", "card", "split", "credit", "part", "cheque"}).Draw(t, "tender")
	if !want.IsPositive() {
		how = "cash"
	}
	part := func() decimal.Decimal {
		return decimal.New(int64(rapid.IntRange(0, int(want.Shift(2).IntPart())).Draw(t, "part")), -2)
	}
	switch how {
	case "cash":
		cashIn = want.Add(decimal.NewFromInt(int64(rapid.IntRange(0, 1000).Draw(t, "extra"))))
		body["payments"] = []pay{cash(cashIn.String())}
	case "card":
		body["payments"] = []pay{{"method": "card", "amount": want.String(), "accountId": m.s.cardAcct}}
	case "split":
		c := part()
		cashIn = want.Sub(c)
		body["payments"] = []pay{{"method": "card", "amount": c.String(), "accountId": m.s.cardAcct}, cash(cashIn.String())}
	case "credit", "part":
		c := decimal.Zero
		if how == "part" {
			c = part()
		}
		cashIn, sale.customer, sale.credit = c, true, want.Sub(c)
		body["customerId"] = m.s.customer
		body["payments"] = []pay{cash(c.String()), {"method": "credit", "amount": want.Sub(c).String()}}
	case "cheque":
		sale.customer, sale.chequeAmt = true, want
		body["customerId"] = m.s.customer
		body["payments"] = []pay{{"method": "cheque", "amount": want.String(), "cheque": map[string]string{"bank": "BOC", "number": "1", "date": "2026-12-01"}}}
	}

	var r saleResult
	if err := m.s.till1.Do("POST", "/api/v1/till/sales", body, &r); err != nil {
		t.Fatalf("SALE-1 sale refused (our total %s, %s): %v", want, how, err)
	}
	if got := dec(t, r.Total); !got.Equal(want) {
		t.Fatalf("SALE-1 server total %s, the rule book's %s (bill %s %s, %+v)", got, want, bill, billType, items)
	}
	change := cashIn.Sub(want).Sub(decimal.Zero)
	if sale.credit.IsPositive() || how == "card" || how == "cheque" {
		change = decimal.Zero
	}
	if how == "split" {
		change = decimal.Zero
	}
	if got := dec(t, r.ChangeGiven); !got.Equal(change) {
		t.Fatalf("TEND-2 change %s, want %s (%s)", got, change, how)
	}
	m.drawer = m.drawer.Add(cashIn).Sub(change)
	m.balance = m.balance.Add(sale.credit)
	sale.id, sale.paid = r.ID, want.Sub(sale.credit)
	if how == "cheque" {
		m.s.must(t, m.s.db.Get(&sale.cheque, `SELECT id FROM cheques WHERE sale_id = $1`, r.ID))
	}
	v := m.s.sale(t, m.s.till1, r.ID)
	for i, l := range v.Lines {
		sale.lines = append(sale.lines, mLine{id: l.ID, qty: dec(t, l.Quantity), left: dec(t, l.Quantity), group: items[i].group, sets: items[i].sets})
	}
	m.sales = append(m.sales, sale)
}

func (m *machine) giveBack(t *rapid.T) {
	var open []*mSale
	for _, s := range m.sales {
		for _, l := range s.lines {
			if l.left.IsPositive() {
				open = append(open, s)
				break
			}
		}
	}
	if len(open) == 0 {
		t.Skip("nothing to return")
	}
	s := rapid.SampledFrom(open).Draw(t, "sale")
	var lines []map[string]any
	picked := map[int64]decimal.Decimal{}
	setsBack := -1
	for i, l := range s.lines {
		if !l.left.IsPositive() {
			continue
		}
		var q decimal.Decimal
		if l.group > 0 {
			if setsBack < 0 {
				out := l.left.Mul(decimal.NewFromInt(int64(l.sets))).Div(l.qty).IntPart()
				setsBack = rapid.IntRange(0, int(out)).Draw(t, "setsBack")
			}
			q = l.qty.Mul(decimal.NewFromInt(int64(setsBack))).Div(decimal.NewFromInt(int64(l.sets)))
		} else if l.left.Equal(l.left.Floor()) {
			q = decimal.NewFromInt(int64(rapid.IntRange(0, int(l.left.IntPart())).Draw(t, fmt.Sprint("back", i))))
		} else {
			q = decimal.New(int64(rapid.IntRange(0, int(l.left.Shift(3).IntPart())).Draw(t, fmt.Sprint("backKg", i))), -3)
		}
		if q.IsPositive() {
			lines = append(lines, map[string]any{"saleItemId": l.id, "quantity": q.String()})
			picked[l.id] = q
		}
	}
	if len(lines) == 0 {
		t.Skip("nothing picked")
	}
	chequeBack := s.cheque != 0 && rapid.Bool().Draw(t, "chequeBack")

	before := m.refundLeft(t, s.id)
	var res struct{ CashRefund string }
	if err := m.s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", s.id), map[string]any{"lines": lines, "chequeBack": chequeBack}, &res); err != nil {
		t.Fatalf("return refused: %v (%v)", err, lines)
	}
	value := before.Sub(m.refundLeft(t, s.id))

	// RET-18/19: debt still owed on the sale (never more than they owe now)
	// first; RET-17: then the cheque given back; the rest is cash.
	rest := value
	if s.customer {
		owed := decimal.Max(decimal.Zero, decimal.Min(s.total.Sub(s.paid).Sub(s.cleared), m.balance))
		clear := decimal.Min(rest, owed)
		m.balance, s.cleared, rest = m.balance.Sub(clear), s.cleared.Add(clear), rest.Sub(clear)
	}
	if chequeBack {
		m.balance = m.balance.Add(s.chequeAmt)
		clear := decimal.Min(rest, s.chequeAmt)
		m.balance, s.cleared, rest = m.balance.Sub(clear), s.cleared.Add(clear), rest.Sub(clear)
		s.cheque = 0
	}
	if got := dec(t, res.CashRefund); !got.Equal(rest) {
		t.Fatalf("RET-15 cash refund %s, the rule book's %s (value %s)", got, rest, value)
	}
	m.drawer = m.drawer.Sub(rest)
	s.refunded = s.refunded.Add(value)
	for i := range s.lines {
		s.lines[i].left = s.lines[i].left.Sub(picked[s.lines[i].id])
	}
}

// refundLeft is what the sale can still refund (the return screen's figures).
func (m *machine) refundLeft(t *rapid.T, id int64) decimal.Decimal {
	var v struct {
		Lines []struct{ RefundLeft string } `json:"lines"`
	}
	m.s.must(t, m.s.till1.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", id), nil, &v))
	sum := decimal.Zero
	for _, l := range v.Lines {
		sum = sum.Add(dec(t, l.RefundLeft))
	}
	return sum
}

func (m *machine) customerPays(t *rapid.T) {
	if !m.balance.IsPositive() {
		t.Skip("owes nothing")
	}
	amt := decimal.New(int64(rapid.IntRange(1, int(m.balance.Shift(2).IntPart())).Draw(t, "pay")), -2)
	m.s.must(t, m.s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", m.s.customer), map[string]any{"amount": amt.String(), "method": "cash"}, nil))
	m.drawer, m.balance = m.drawer.Add(amt), m.balance.Sub(amt)
}

func (m *machine) bounce(t *rapid.T) {
	var pending []*mSale
	for _, s := range m.sales {
		if s.cheque != 0 {
			pending = append(pending, s)
		}
	}
	if len(pending) == 0 {
		t.Skip("no cheque waiting")
	}
	s := rapid.SampledFrom(pending).Draw(t, "cheque")
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", s.cheque), map[string]string{"status": "bounced"}, nil))
	m.balance, s.cheque = m.balance.Add(s.chequeAmt), 0
}

func (m *machine) closeOpen(t *rapid.T) {
	m.s.close(t, m.s.till1)
	m.s.open(t, m.s.till1)
	m.drawer = decimal.NewFromInt(5000)
}

// check: the auditor, then the machine's books against the server's.
func (m *machine) check(t *rapid.T) {
	m.s.audit(t)
	var st struct {
		Summary struct{ Expected string } `json:"summary"`
	}
	m.s.must(t, m.s.till1.Do("GET", "/api/v1/till/drawer", nil, &st))
	if got := dec(t, st.Summary.Expected); !got.Equal(m.drawer) {
		t.Fatalf("DRAW-1 drawer expects %s, the books say %s", got, m.drawer)
	}
	var bal decimal.Decimal
	m.s.must(t, m.s.db.Get(&bal, `SELECT outstanding_balance FROM customers WHERE id = $1`, m.s.customer))
	if !bal.Equal(m.balance) {
		t.Fatalf("TEND-11 customer owes %s, the books say %s", bal, m.balance)
	}
	for _, s := range m.sales {
		if s.refunded.GreaterThan(s.total) {
			t.Fatalf("RET-5 sale %d refunded %s of %s", s.id, s.refunded, s.total)
		}
		done := true
		for _, l := range s.lines {
			done = done && !l.left.IsPositive()
		}
		if done && !s.refunded.Equal(s.total) {
			t.Fatalf("RET-5 sale %d fully back, refunded %s of %s", s.id, s.refunded, s.total)
		}
	}
}
