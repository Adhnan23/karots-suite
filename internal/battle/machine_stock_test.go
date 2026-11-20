package battle

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Random shop days with stock (docs/rules/stock.md): lots in and topped up, counts
// of a product and of one lot, sales by FEFO and by picked lot, write-offs (each
// reason, named lot or FEFO, a frozen item off the shelf), give backs with a
// different count, recalls, bags opened into loose rice (some weighed out short),
// lots merged, freeze → put out for sale → unfreeze. The machine keeps every lot
// by the rule book (good lots first, STK-21); after every step the auditor runs and
// each lot and each product's on hand are held to the books (STK-1, STK-2).
func TestBattleStock(t *testing.T) {
	t.Parallel()
	setRapid(t)
	rapid.Check(t, func(rt *rapid.T) {
		m := newStockMachine(rt)
		rt.Repeat(map[string]func(*rapid.T){
			"":         m.check,
			"Intake":   m.intake,
			"TopUp":    m.topUp,
			"Count":    m.count,
			"LotCount": m.lotCount,
			"Sell":     m.sell,
			"WriteOff": m.writeOff,
			"GiveBack": m.giveBack,
			"Recall":   m.recall,
			"Convert":  m.convert,
			"Merge":    m.merge,
			"Freeze":   m.freeze,
		})
		m.s.agreeToday(rt)
	})
}

type mLot struct {
	id       int64
	qty      decimal.Decimal
	exp      string // YYYY-MM-DD or ""
	recalled bool
	stops    bool // recalled and the shop stopped selling it
}

type mProd struct {
	name    string
	id      int64
	decimal bool
	frozen  bool
	lots    []*mLot
}

func (p *mProd) total() decimal.Decimal {
	t := decimal.Zero
	for _, l := range p.lots {
		t = t.Add(l.qty)
	}
	return t
}

func (p *mProd) live() []*mLot {
	var out []*mLot
	for _, l := range p.lots {
		if l.qty.IsPositive() {
			out = append(out, l)
		}
	}
	return out
}

type stockMachine struct {
	s                       *shop
	sup, conv               int64
	pills, bag, rice, pencl *mProd
	today                   time.Time
}

func newStockMachine(t *rapid.T) *stockMachine {
	s := newShop(t)
	m := &stockMachine{s: s, today: time.Now().UTC().Truncate(24 * time.Hour)}
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Machine Supplier", "phone": "0112000099"}, &sup))
	m.sup = sup.ID
	mk := func(name string, unit, supplier int64, dec bool) *mProd {
		return &mProd{name: name, id: s.stocked(t, name, unit, supplier, nil), decimal: dec}
	}
	m.pills, m.bag, m.rice, m.pencl = mk("M pills", s.unitPcs, m.sup, false), mk("M bag", s.unitPcs, 0, false),
		mk("M rice", s.unitKg, 0, true), mk("M pencil", s.unitPcs, 0, false)
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": m.bag.id, "toProductId": m.rice.id, "ratio": "25"}, &cv))
	m.conv = cv.ID
	s.open(t, s.till1)
	return m
}

func (m *stockMachine) prod(t *rapid.T, from ...*mProd) *mProd {
	if len(from) == 0 {
		from = []*mProd{m.pills, m.bag, m.rice, m.pencl}
	}
	return rapid.SampledFrom(from).Draw(t, "product")
}

// qty draws an amount up to max (a tenth for a decimal product), at least min.
func qty(t *rapid.T, p *mProd, min, max int, label string) decimal.Decimal {
	if p.decimal {
		return decimal.New(int64(rapid.IntRange(min*10, max*10).Draw(t, label)), -1)
	}
	return decimal.NewFromInt(int64(rapid.IntRange(min, max).Draw(t, label)))
}

func (m *stockMachine) expired(l *mLot) bool {
	return l.exp != "" && l.exp < m.today.Format(time.DateOnly)
}

// fefo is the order a removal with no lot named takes: good lots first, earliest
// expiry first (none last), then the oldest (STK-21).
func (m *stockMachine) fefo(p *mProd) []*mLot {
	ls := p.live()
	bad := func(l *mLot) bool { return m.expired(l) || l.stops }
	slices.SortStableFunc(ls, func(a, b *mLot) int {
		switch {
		case bad(a) != bad(b):
			if bad(a) {
				return 1
			}
			return -1
		case a.exp != b.exp:
			if a.exp == "" {
				return 1
			}
			if b.exp == "" {
				return -1
			}
			if a.exp < b.exp {
				return -1
			}
			return 1
		}
		return int(a.id - b.id)
	})
	return ls
}

func (m *stockMachine) take(p *mProd, q decimal.Decimal) {
	for _, l := range m.fefo(p) {
		n := decimal.Min(l.qty, q)
		l.qty, q = l.qty.Sub(n), q.Sub(n)
	}
}

// adopt picks up the lots an action opened, and says how much they hold.
func (m *stockMachine) adopt(t *rapid.T, p *mProd) decimal.Decimal {
	var last int64
	for _, l := range p.lots {
		last = max(last, l.id)
	}
	var rows []struct {
		ID  int64           `db:"id"`
		Qty decimal.Decimal `db:"qty_remaining"`
		Exp string          `db:"exp"`
	}
	m.s.must(t, m.s.db.Select(&rows, `SELECT id, qty_remaining, COALESCE(expiry_date::text, '') AS exp FROM stock_batches
		WHERE product_id = $1 AND id > $2 ORDER BY id`, p.id, last))
	got := decimal.Zero
	for _, r := range rows {
		p.lots = append(p.lots, &mLot{id: r.ID, qty: r.Qty, exp: r.Exp})
		got = got.Add(r.Qty)
	}
	return got
}

func (m *stockMachine) try(t *rapid.T, ok bool, rule, what string, call func() error) bool {
	err := call()
	if ok && err != nil {
		t.Fatalf("%s %s refused: %v", rule, what, err)
	}
	if !ok && err == nil {
		t.Fatalf("%s %s went through; the books say it can't", rule, what)
	}
	return err == nil
}

func (m *stockMachine) lot(t *rapid.T, p *mProd) *mLot {
	ls := p.live()
	if len(ls) == 0 {
		t.Skip("no lot")
	}
	return rapid.SampledFrom(ls).Draw(t, "lot")
}

func (m *stockMachine) intake(t *rapid.T) {
	p := m.prod(t)
	q := qty(t, p, 1, 20, "qty")
	exp := rapid.SampledFrom([]string{"", m.today.AddDate(0, 0, -30).Format(time.DateOnly),
		m.today.AddDate(0, 2, 0).Format(time.DateOnly), m.today.AddDate(0, 7, 0).Format(time.DateOnly)}).Draw(t, "expiry")
	cost := fmt.Sprint(rapid.IntRange(1, 60).Draw(t, "cost"))
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/add", p.id), map[string]string{"qty": q.String(), "cost": cost, "expiry": exp}, nil))
	if got := m.adopt(t, p); !got.Equal(q) {
		t.Fatalf("STK-6 intake of %s opened lots holding %s", q, got)
	}
}

func (m *stockMachine) topUp(t *rapid.T) {
	p := m.prod(t)
	l := m.lot(t, p)
	n := qty(t, p, 1, 5, "more")
	m.s.must(t, m.s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p.id, "newQuantity": p.total().Add(n).String(), "batchId": l.id}, nil))
	l.qty = l.qty.Add(n)
}

func (m *stockMachine) count(t *rapid.T) {
	p := m.prod(t)
	target := qty(t, p, 0, int(p.total().IntPart())+5, "counted")
	delta := target.Sub(p.total())
	m.s.must(t, m.s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p.id, "newQuantity": target.String(), "note": "machine count"}, nil))
	if delta.IsNegative() {
		m.take(p, delta.Neg())
	} else if got := m.adopt(t, p); !got.Equal(delta) {
		t.Fatalf("STK-4 a count %s up opened lots holding %s", delta, got)
	}
}

func (m *stockMachine) lotCount(t *rapid.T) {
	p := m.prod(t)
	l := m.lot(t, p)
	n := qty(t, p, 0, int(l.qty.IntPart())+3, "lot counted")
	m.s.must(t, m.s.admin.Do("POST", "/api/v1/admin/stock/take", map[string]any{"lines": []map[string]any{{"productId": p.id,
		"batches": []map[string]any{{"batchId": l.id, "counted": n.String()}}}}}, nil))
	l.qty = n
}

func (m *stockMachine) sell(t *rapid.T) {
	p := m.prod(t, m.pills, m.rice, m.pencl)
	q := qty(t, p, 1, 5, "qty")
	it := line{"productId": p.id, "quantity": q.String()}
	var picked *mLot
	if ls := p.live(); len(ls) > 0 && rapid.Bool().Draw(t, "pick a lot") {
		picked = rapid.SampledFrom(ls).Draw(t, "picked")
		it["batchId"] = picked.id
	}
	ok := p.frozen || (picked == nil && !q.GreaterThan(p.total())) || (picked != nil && !q.GreaterThan(picked.qty))
	if !m.try(t, ok, "SALE-22", "sale of "+q.String(), func() error {
		return m.s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": []line{it}, "payments": []pay{cash(q.Mul(decimal.NewFromInt(20)).StringFixed(2))}}, nil)
	}) || p.frozen {
		return
	}
	if picked != nil {
		picked.qty = picked.qty.Sub(q)
	} else {
		m.take(p, q)
	}
}

func (m *stockMachine) writeOff(t *rapid.T) {
	p := m.prod(t)
	q := qty(t, p, 1, 3, "qty")
	body := map[string]any{"productId": p.id, "quantity": q.String(), "reason": rapid.SampledFrom([]string{"damage", "own_use", "staff"}).Draw(t, "reason")}
	var named *mLot
	if ls := p.live(); len(ls) > 0 && rapid.Bool().Draw(t, "name a lot") {
		named = rapid.SampledFrom(ls).Draw(t, "named")
		body["batchId"] = named.id
	}
	shelf := p.frozen && named != nil && rapid.Bool().Draw(t, "off the shelf")
	body["fromShelf"] = shelf
	ok := shelf || (named == nil && !q.GreaterThan(p.total())) || (named != nil && !q.GreaterThan(named.qty))
	if !m.try(t, ok, "STK-3", "write-off of "+q.String(), func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/stock/damage", body, nil)
	}) || shelf {
		return // off the shelf: brought in, then written off — the lot is as it was (STK-8)
	}
	if named != nil {
		named.qty = named.qty.Sub(q)
	} else {
		m.take(p, q)
	}
}

// giveBack hands a pills lot back with a count of its own (STK-9).
func (m *stockMachine) giveBack(t *rapid.T) {
	p := m.pills
	l := m.lot(t, p)
	c := decimal.NewFromInt(int64(rapid.IntRange(0, int(l.qty.IntPart())+3).Draw(t, "counted")))
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/give-back", m.sup, l.id), map[string]string{"counted": c.String()}, nil))
	var others []*mLot // next to sell first, as the count settles them
	for _, o := range p.live() {
		if o != l {
			others = append(others, o)
		}
	}
	slices.SortStableFunc(others, func(a, b *mLot) int {
		if a.exp != b.exp {
			switch {
			case a.exp == "":
				return 1
			case b.exp == "":
				return -1
			case a.exp < b.exp:
				return -1
			}
			return 1
		}
		return int(a.id - b.id)
	})
	switch diff := c.Sub(l.qty); {
	case diff.IsPositive():
		for _, o := range others {
			n := decimal.Min(diff, o.qty)
			o.qty, diff = o.qty.Sub(n), diff.Sub(n)
		}
	case diff.IsNegative() && len(others) > 0:
		others[0].qty = others[0].qty.Add(diff.Neg())
	}
	l.qty = decimal.Zero // the count is handed over
}

func (m *stockMachine) recall(t *rapid.T) {
	l := m.lot(t, m.pills)
	url := fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", m.sup, l.id)
	if l.recalled {
		m.s.must(t, m.s.admin.Do("DELETE", url, nil, nil))
		l.recalled, l.stops = false, false
		return
	}
	stop := rapid.Bool().Draw(t, "stop selling")
	m.s.must(t, m.s.admin.Do("POST", url, map[string]bool{"stopSelling": stop}, nil))
	l.recalled, l.stops = true, stop
}

func (m *stockMachine) convert(t *rapid.T) {
	n := decimal.NewFromInt(int64(rapid.IntRange(1, 2).Draw(t, "bags")))
	want := n.Mul(decimal.NewFromInt(25))
	body := map[string]string{"quantity": n.String()}
	if rapid.Bool().Draw(t, "weighed short") {
		want = want.Sub(decimal.New(int64(rapid.IntRange(1, 20).Draw(t, "short")), -1))
		body["toQty"] = want.String()
	}
	if !m.try(t, !n.GreaterThan(m.bag.total()), "STK-3", "opening "+n.String()+" bags", func() error {
		return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", m.conv), body, nil)
	}) {
		return
	}
	m.take(m.bag, n)
	if got := m.adopt(t, m.rice); !got.Equal(want) {
		t.Fatalf("STK-11 %s bags gave loose lots holding %s, want %s", n, got, want)
	}
}

func (m *stockMachine) merge(t *rapid.T) {
	p := m.prod(t)
	ls := p.live()
	if len(ls) < 2 {
		t.Skip("one lot")
	}
	i := rapid.IntRange(0, len(ls)-1).Draw(t, "a")
	j := rapid.IntRange(0, len(ls)-2).Draw(t, "b")
	if j >= i {
		j++
	}
	a, b := ls[i], ls[j]
	if a.id > b.id {
		a, b = b, a // the oldest is kept
	}
	exp := a.exp
	if b.exp != "" && (exp == "" || b.exp < exp) {
		exp = b.exp
	}
	if !m.try(t, !a.recalled && !b.recalled, "STK-15", "merging two lots", func() error {
		return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/merge", p.id), map[string]any{"batchIds": []int64{a.id, b.id}, "expiry": exp}, nil)
	}) {
		return
	}
	a.qty, b.qty, a.exp = a.qty.Add(b.qty), decimal.Zero, exp
}

// freeze moves the pencils through a freeze: frozen, boxes put out, unfrozen with
// what's out for sale (STK-12).
func (m *stockMachine) freeze(t *rapid.T) {
	p := m.pencl
	if !p.frozen {
		m.s.must(t, m.s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p.id}}, nil))
		p.frozen = true
		return
	}
	if rapid.Bool().Draw(t, "put out for sale") {
		q := qty(t, p, 1, 10, "out")
		if m.try(t, !q.GreaterThan(p.total()), "STK-3", "putting out "+q.String(), func() error {
			return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", p.id), map[string]string{"quantity": q.String()}, nil)
		}) {
			m.take(p, q)
		}
		return
	}
	q := qty(t, p, 0, 15, "on the shelf")
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/unfreeze", p.id), map[string]string{"shelfQty": q.String()}, nil))
	p.frozen = false
	if got := m.adopt(t, p); !got.Equal(q) {
		t.Fatalf("STK-12 unfreezing with %s out opened lots holding %s", q, got)
	}
}

func (m *stockMachine) check(t *rapid.T) {
	m.s.audit(t)
	m.s.agreeSometimes(t)
	for _, p := range []*mProd{m.pills, m.bag, m.rice, m.pencl} {
		var on decimal.Decimal
		m.s.must(t, m.s.db.Get(&on, `SELECT quantity FROM stock WHERE product_id = $1`, p.id))
		if !on.Equal(p.total()) {
			t.Fatalf("STK-1 %s on hand %s, the books say %s", p.name, on, p.total())
		}
		for _, l := range p.lots {
			var q decimal.Decimal
			m.s.must(t, m.s.db.Get(&q, `SELECT qty_remaining FROM stock_batches WHERE id = $1`, l.id))
			if !q.Equal(l.qty) {
				t.Fatalf("STK-2 %s lot %d holds %s, the books say %s", p.name, l.id, q, l.qty)
			}
		}
	}
}
