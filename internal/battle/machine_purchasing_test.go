package battle

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Random shop days buying from and paying two suppliers (docs/rules/purchasing.md):
// deliveries with no order (free units, line and bill discounts, packs, loose
// goods) and orders received in part, paid on delivery or not; payments oldest
// first / old debt first / a typed advance, by cash, card, from outside, by
// cheque, or a discount; goods sent back; refunds; cheques cleared or bounced;
// the opening adjusted; a supplier disabled and brought back. The machine keeps
// every balance, old debt and bill by the rule book; after every step the
// auditor runs and the shop is held to the books (SUP-1, SUP-2, PUR-11, PUR-12).
func TestBattlePurchasing(t *testing.T) {
	t.Parallel()
	setRapid(t)
	rapid.Check(t, func(rt *rapid.T) {
		m := newPurchasingMachine(rt)
		rt.Repeat(map[string]func(*rapid.T){
			"":        m.check,
			"Deliver": m.deliver,
			"Receive": m.receive,
			"Pay":     m.pay,
			"Return":  m.sendBack,
			"Refund":  m.refund,
			"Cheque":  m.cheque,
			"Opening": m.opening,
			"Disable": m.disable,
		})
	})
}

type pCheque struct {
	id     int64
	amount decimal.Decimal
	sup    *pSup
	pay    *pPay
}

type purchasingMachine struct {
	s          *shop
	safe, bank int64
	sups       []*pSup
	cheques    []*pCheque
	n          int // cheque numbers
}

func newPurchasingMachine(t *rapid.T) *purchasingMachine {
	s := newShop(t)
	m := &purchasingMachine{s: s, safe: s.locker(t, "M safe", "safe", "10000000"), bank: s.locker(t, "M bank", "bank", "10000000")}
	for _, x := range []struct{ name, opening string }{{"M Wholesale", "500"}, {"M Agency", ""}} {
		body := map[string]string{"name": x.name}
		if x.opening != "" {
			body["openingBalance"] = x.opening
		}
		var r struct{ ID int64 }
		s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", body, &r))
		o := decimal.RequireFromString("0" + x.opening)
		m.sups = append(m.sups, &pSup{id: r.ID, name: x.name, bal: o, old: o, active: true})
	}
	return m
}

func (m *purchasingMachine) sup(t *rapid.T) *pSup {
	return rapid.SampledFrom(m.sups).Draw(t, "supplier")
}

func (m *purchasingMachine) try(t *rapid.T, ok bool, rule, what string, call func() error) bool {
	err := call()
	if ok && err != nil {
		t.Fatalf("%s %s refused: %v", rule, what, err)
	}
	if !ok && err == nil {
		t.Fatalf("%s %s went through; the books say it can't", rule, what)
	}
	return err == nil
}

func cents(t *rapid.T, lo, hi int, label string) decimal.Decimal {
	return decimal.New(int64(rapid.IntRange(lo, hi).Draw(t, label)), -2)
}

// dLine is one delivery line as drawn: what's sent and what the books make of it.
type dLine struct {
	body    sl
	product int64
	units   decimal.Decimal // paid + free, in units
	net     decimal.Decimal
	whole   bool
}

func (m *purchasingMachine) lines(t *rapid.T) []dLine {
	var out []dLine
	for i := range rapid.IntRange(1, 3).Draw(t, "lines") {
		kind := rapid.SampledFrom([]string{"plain", "odd", "pack", "loose"}).Draw(t, fmt.Sprint("kind", i))
		q := decimal.NewFromInt(int64(rapid.IntRange(1, 12).Draw(t, "qty")))
		free := decimal.NewFromInt(int64(rapid.IntRange(0, 2).Draw(t, "free")))
		if kind == "loose" {
			q = decimal.New(int64(rapid.IntRange(5, 120).Draw(t, "kg")), -1)
		}
		cost := cents(t, 100, 6000, "cost")
		l := dLine{product: m.s.prod[kind], whole: kind != "loose",
			body: sl{"productId": m.s.prod[kind], "quantity": q.String(), "freeQty": free.String(), "costPrice": cost.String()}}
		disc, percent := zero, false
		switch rapid.IntRange(0, 2).Draw(t, "line discount") {
		case 1:
			disc = cents(t, 0, 2000, "per unit off")
			l.body["discount"], l.body["discountType"] = disc.String(), "fixed"
		case 2:
			disc, percent = decimal.NewFromInt(int64(rapid.IntRange(0, 110).Draw(t, "percent off"))), true
			l.body["discount"], l.body["discountType"] = disc.String(), "percent"
		}
		l.net = lineNet(q, cost, disc, percent)
		l.units = q.Add(free)
		if kind == "pack" {
			l.body["packSize"] = "12"
			l.units = l.units.Mul(decimal.NewFromInt(12))
		}
		out = append(out, l)
	}
	return out
}

// landed books a bill the shop just took: its lines' lots (cost read from the
// lot — PUR-3 is the cases' job), the total, settlement, then pay on delivery.
func (m *purchasingMachine) landed(t *rapid.T, sp *pSup, id int64, ls []dLine, sub, disc decimal.Decimal, percent bool, payNow decimal.Decimal) {
	b := &pBill{id: id, total: billTotal(sub, disc, percent)}
	for _, l := range ls {
		var lot pLot
		m.s.must(t, m.s.db.QueryRow(`SELECT b.id, b.qty_received, b.cost_price FROM stock_batches b JOIN purchase_items pi ON pi.id = b.purchase_item_id
			WHERE pi.purchase_id = $1 AND b.product_id = $2 AND b.id <> ALL($3::bigint[]) ORDER BY b.id LIMIT 1`, id, l.product, m.lotIDs(b)).Scan(&lot.id, &lot.left, &lot.cost))
		if !lot.left.Equal(l.units) {
			t.Fatalf("PUR-2 a line of %s units opened a lot of %s", l.units, lot.left)
		}
		lot.product, lot.whole = l.product, l.whole
		b.lots = append(b.lots, &lot)
	}
	sp.book(b)
	if paid := decimal.Min(payNow, b.left()); paid.IsPositive() {
		sp.pay(plan{allocs: []*pAlloc{{bill: b, amt: paid}}}, paid, false)
	}
}

func (m *purchasingMachine) lotIDs(b *pBill) string {
	ids := "{0"
	for _, l := range b.lots {
		ids += fmt.Sprintf(",%d", l.id)
	}
	return ids + "}"
}

// billDisc draws a bill discount and adds it to the request body.
func billDisc(t *rapid.T, body map[string]any) (decimal.Decimal, bool) {
	switch rapid.IntRange(0, 2).Draw(t, "bill discount") {
	case 1:
		d := cents(t, 0, 50000, "bill off")
		body["discount"], body["discountType"] = d.String(), "fixed"
		return d, false
	case 2:
		d := decimal.NewFromInt(int64(rapid.IntRange(0, 30).Draw(t, "bill percent")))
		body["discount"], body["discountType"] = d.String(), "percent"
		return d, true
	}
	return zero, false
}

func (m *purchasingMachine) payNow(t *rapid.T, body map[string]any) decimal.Decimal {
	if !rapid.Bool().Draw(t, "pay on delivery") {
		return zero
	}
	amt := cents(t, 100, 300000, "pay now")
	body["pay"] = map[string]string{"amount": amt.String(), "method": "cash", "source": fmt.Sprintf("locker:%d", m.safe)}
	return amt
}

func sum(ls []dLine) decimal.Decimal {
	s := zero
	for _, l := range ls {
		s = s.Add(l.net)
	}
	return s
}

func (m *purchasingMachine) deliver(t *rapid.T) {
	sp := m.sup(t)
	ls := m.lines(t)
	body := map[string]any{"supplierId": sp.id}
	for _, l := range ls {
		body["lines"] = append(asLines(body["lines"]), l.body)
	}
	disc, percent := billDisc(t, body)
	now := m.payNow(t, body)
	var r struct{ PurchaseID int64 }
	if m.try(t, sp.active, "SUP-9", "a delivery from "+sp.name, func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/purchases/receive", body, &r)
	}) {
		m.landed(t, sp, r.PurchaseID, ls, sum(ls), disc, percent, now)
	}
}

func asLines(v any) []sl {
	if v == nil {
		return nil
	}
	return v.([]sl)
}

// receive orders goods, then receives what came — some short, some not at all,
// sometimes keeping the rest on order (PUR-5, PUR-6).
func (m *purchasingMachine) receive(t *rapid.T) {
	sp := m.sup(t)
	ls := m.lines(t)
	var order []sl
	for _, l := range ls {
		order = append(order, sl{"productId": l.product, "quantity": "20", "costPrice": "1"})
	}
	var o struct{ ID int64 }
	if !m.try(t, sp.active, "SUP-9", "an order from "+sp.name, func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sp.id, "lines": order}, &o)
	}) {
		return
	}
	came := ls[:rapid.IntRange(1, len(ls)).Draw(t, "lines that came")]
	body := map[string]any{"supplierId": sp.id, "keepRemainder": rapid.Bool().Draw(t, "keep the rest")}
	for _, l := range came {
		body["lines"] = append(asLines(body["lines"]), l.body)
	}
	disc, percent := billDisc(t, body)
	now := m.payNow(t, body)
	path := fmt.Sprintf("/api/v1/admin/purchases/%d/receive", o.ID)
	m.s.must(t, m.s.admin.Do("POST", path, body, nil))
	m.landed(t, sp, o.ID, came, sum(came), disc, percent, now)
	m.try(t, false, "PUR-6", "an order received twice", func() error { return m.s.admin.Do("POST", path, body, nil) })
}

// pay pays a supplier: one amount (bills or old debt first) or a typed advance,
// by cash, card, from outside, by cheque, or a discount.
func (m *purchasingMachine) pay(t *rapid.T) {
	sp := m.sup(t)
	amt := cents(t, 100, 400000, "amount")
	method := rapid.SampledFrom([]string{"cash", "card", "online", "cheque", "discount"}).Draw(t, "method")
	mode := rapid.SampledFrom([]string{"pay", "old", "advance"}).Draw(t, "mode")
	body := map[string]any{"method": method, "mode": mode, "amount": amt.String()}
	if mode == "advance" {
		body = map[string]any{"method": method, "advance": amt.String()}
		mode = "pay"
	}
	switch method {
	case "cash":
		body["source"] = fmt.Sprintf("locker:%d", m.safe)
	case "card":
		body["source"] = fmt.Sprintf("locker:%d", m.bank)
	case "cheque":
		m.n++
		body["cheques"] = []map[string]any{{"lockerId": m.bank, "number": fmt.Sprintf("9%05d", m.n), "amount": amt.String(), "date": time.Now().Format(time.DateOnly)}}
	}
	pl := sp.distribute(amt, mode)
	ok := method != "discount" || !pl.rest.IsPositive()
	if !m.try(t, ok, "SUP-6", "a discount of "+amt.String(), func() error { return m.s.admin.Do("POST", supPath(sp.id, "payment"), body, nil) }) {
		return
	}
	p := sp.pay(pl, amt, method == "discount")
	if method == "cheque" {
		var id int64
		m.s.must(t, m.s.db.Get(&id, `SELECT id FROM cheques WHERE number = $1`, fmt.Sprintf("9%05d", m.n)))
		m.cheques = append(m.cheques, &pCheque{id: id, amount: amt, sup: sp, pay: p})
	}
}

// sendBack sends part of a delivery's lot back (sometimes more than it holds).
func (m *purchasingMachine) sendBack(t *rapid.T) {
	sp := m.sup(t)
	var lots []*pLot
	var own []*pBill
	for _, b := range sp.bills {
		for _, l := range b.lots {
			if l.left.IsPositive() {
				lots, own = append(lots, l), append(own, b)
			}
		}
	}
	if len(lots) == 0 {
		t.Skip("nothing to send back")
	}
	i := rapid.IntRange(0, len(lots)-1).Draw(t, "lot")
	l, b := lots[i], own[i]
	q := decimal.NewFromInt(int64(rapid.IntRange(1, int(l.left.Ceil().IntPart())+2).Draw(t, "qty")))
	if !l.whole && rapid.Bool().Draw(t, "a part of a kg") {
		q = q.Sub(decimal.New(5, -1))
	}
	ok := sp.active && !q.GreaterThan(l.left)
	if m.try(t, ok, "PUR-9", "sending back "+q.String()+" of "+l.left.String(), func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sp.id, "purchaseId": b.id,
			"lines": []map[string]any{{"productId": l.product, "quantity": q.String(), "batchId": l.id}}}, nil)
	}) {
		l.left = l.left.Sub(q)
		sp.sendBack(b, q.Mul(l.cost).Round(2))
	}
}

func (m *purchasingMachine) refund(t *rapid.T) {
	sp := m.sup(t)
	credit := decimal.Max(sp.bal.Neg(), zero)
	amt := cents(t, 1, int(credit.Mul(decimal.NewFromInt(100)).IntPart())+500, "refund")
	method, place := "cash", m.safe
	if rapid.Bool().Draw(t, "into the bank") {
		method, place = "online", m.bank
	}
	if m.try(t, !amt.GreaterThan(credit), "SUP-7", "a refund of "+amt.String()+" (credit "+credit.String()+")", func() error {
		return m.s.admin.Do("POST", supPath(sp.id, "refund"), map[string]string{"amount": amt.String(), "method": method, "source": fmt.Sprintf("locker:%d", place)}, nil)
	}) {
		sp.addBal(amt)
	}
}

// cheque clears or bounces / cancels a pending given cheque.
func (m *purchasingMachine) cheque(t *rapid.T) {
	if len(m.cheques) == 0 {
		t.Skip("no cheque pending")
	}
	i := rapid.IntRange(0, len(m.cheques)-1).Draw(t, "cheque")
	c := m.cheques[i]
	m.cheques = append(m.cheques[:i], m.cheques[i+1:]...)
	switch rapid.SampledFrom([]string{"clear", "bounced", "cancelled"}).Draw(t, "what") {
	case "clear":
		m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/clear", c.id), map[string]string{}, nil))
	case "bounced":
		m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", c.id), map[string]string{"status": "bounced"}, nil))
		c.sup.bounce(c.pay, c.amount)
	default:
		m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", c.id), map[string]string{"status": "cancelled"}, nil))
		c.sup.bounce(c.pay, c.amount)
	}
}

func (m *purchasingMachine) opening(t *rapid.T) {
	sp := m.sup(t)
	to := cents(t, 0, 200000, "opening")
	m.s.must(t, m.s.admin.Do("POST", supPath(sp.id, "opening"), map[string]string{"opening": to.String(), "note": "machine"}, nil))
	sp.adjustOpening(to)
}

func (m *purchasingMachine) disable(t *rapid.T) {
	sp := m.sup(t)
	if sp.active {
		m.s.must(t, m.s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/suppliers/%d", sp.id), nil, nil))
	} else {
		m.s.must(t, m.s.admin.Do("POST", supPath(sp.id, "activate"), map[string]string{}, nil))
	}
	sp.active = !sp.active
}

func (m *purchasingMachine) check(t *rapid.T) {
	m.s.audit(t)
	for _, sp := range m.sups {
		var bal, old decimal.Decimal
		m.s.must(t, m.s.db.QueryRow(`SELECT outstanding_balance, opening_unlinked FROM suppliers WHERE id = $1`, sp.id).Scan(&bal, &old))
		if !bal.Equal(sp.bal) || !old.Equal(sp.old) {
			t.Fatalf("SUP-1 %s owed %s (old debt %s), the books say %s (%s)", sp.name, bal, old, sp.bal, sp.old)
		}
		for _, b := range sp.bills {
			var paid, credited decimal.Decimal
			m.s.must(t, m.s.db.QueryRow(`SELECT paid_amount, credited_amount FROM purchases WHERE id = $1`, b.id).Scan(&paid, &credited))
			if !paid.Equal(b.paid) || !credited.Equal(b.credited) {
				t.Fatalf("PUR-12 %s bill %d paid %s credited %s, the books say %s and %s (total %s)", sp.name, b.id, paid, credited, b.paid, b.credited, b.total)
			}
		}
	}
}
