package battle

import "github.com/shopspring/decimal"

// The purchasing machine's own books (docs/rules/purchasing.md), worked out by
// the rule book alone — never read back from the shop. The shop is then held to
// them after every step.

var zero = decimal.Zero

type pLot struct {
	id, product int64
	left, cost  decimal.Decimal
	share, back decimal.Decimal // the line's share of the bill; what's gone back so far
	whole       bool
}

// value is what q units of the lot are worth going back: units × cost, but the
// last units take what's left of the line's share (PUR-3, PUR-10, Q5).
func (l *pLot) value(q decimal.Decimal) decimal.Decimal {
	v := q.Mul(l.cost).Round(2)
	if q.Equal(l.left) {
		v = decimal.Max(l.share.Sub(l.back), zero)
	}
	return v
}

// shares splits a bill's total over its lines' nets in cents, the last line
// that cost something taking what's left (PUR-3); a line that cost nothing gets 0.
func shares(nets []decimal.Decimal, total decimal.Decimal) []decimal.Decimal {
	all := zero
	for _, n := range nets {
		all = all.Add(n)
	}
	out := make([]decimal.Decimal, len(nets))
	if !all.IsPositive() {
		return out
	}
	last := 0
	for i, n := range nets {
		if n.IsPositive() {
			last = i
		}
	}
	given := zero
	for i, n := range nets {
		out[i] = n.Mul(total).DivRound(all, 2)
		if i == last {
			out[i] = total.Sub(given)
		}
		given = given.Add(out[i])
	}
	return out
}

type pBill struct {
	id                    int64
	total, paid, credited decimal.Decimal
	lots                  []*pLot
}

func (b *pBill) left() decimal.Decimal { return b.total.Sub(b.paid).Sub(b.credited) }

type pAlloc struct {
	bill          *pBill
	amt, reopened decimal.Decimal
}

// pPay is a payment: its money (or discount) and where it went. back is what
// bounced / cancelled cheques took back (CHQ-3, CHQ-4).
type pPay struct {
	amount, back decimal.Decimal
	discount     bool
	allocs       []*pAlloc
}

// advance is what of the payment no bill has taken and no bounce took back.
func (p *pPay) advance() decimal.Decimal {
	if p.discount {
		return zero
	}
	put, reopened := zero, zero
	for _, a := range p.allocs {
		put, reopened = put.Add(a.amt), reopened.Add(a.reopened)
	}
	return p.amount.Sub(put).Sub(decimal.Max(p.back.Sub(reopened), zero))
}

type pRet struct{ total, allocated decimal.Decimal }

type pSup struct {
	id       int64
	name     string
	bal, old decimal.Decimal
	active   bool
	bills    []*pBill
	pays     []*pPay
	rets     []*pRet
}

func (s *pSup) open() []*pBill {
	var out []*pBill
	for _, b := range s.bills {
		if b.left().IsPositive() {
			out = append(out, b)
		}
	}
	return out
}

// addBal moves the balance; the old debt never passes what they're owed (SUP-2).
func (s *pSup) addBal(d decimal.Decimal) {
	s.bal = s.bal.Add(d)
	s.old = decimal.Min(s.old, decimal.Max(s.bal, zero))
}

// settle spends what the supplier holds for the shop on a new bill: advances
// (paid), then goods-sent-back credits (credited), never past the credit on
// hand before the bill (PUR-8).
func (s *pSup) settle(b *pBill, balBefore decimal.Decimal) {
	want := decimal.Min(b.left(), decimal.Max(balBefore.Neg(), zero))
	for _, p := range s.pays {
		if !want.IsPositive() {
			return
		}
		if take := decimal.Min(p.advance(), want); take.IsPositive() {
			b.paid = b.paid.Add(take)
			p.allocs = append(p.allocs, &pAlloc{bill: b, amt: take})
			want = want.Sub(take)
		}
	}
	for _, r := range s.rets {
		if !want.IsPositive() {
			return
		}
		if take := decimal.Min(r.total.Sub(r.allocated), want); take.IsPositive() {
			b.credited = b.credited.Add(take)
			r.allocated = r.allocated.Add(take)
			want = want.Sub(take)
		}
	}
}

// book adds a delivered bill and settles it from credit (PUR-2, PUR-8).
func (s *pSup) book(b *pBill) {
	before := s.bal
	s.bills = append(s.bills, b)
	if b.total.IsPositive() {
		s.addBal(b.total)
	}
	s.settle(b, before)
}

// plan is a payment's split: on bills, on the old debt, the rest an advance.
type plan struct {
	allocs []*pAlloc
	old    decimal.Decimal
	rest   decimal.Decimal
}

// distribute splits one amount: bills oldest first then old debt ("pay"), or
// the old debt first ("old"); what's left is an advance (SUP-4, SUP-5).
func (s *pSup) distribute(amount decimal.Decimal, mode string) plan {
	var pl plan
	left := amount
	bills := func() {
		for _, b := range s.open() {
			if take := decimal.Min(b.left(), left); take.IsPositive() {
				pl.allocs = append(pl.allocs, &pAlloc{bill: b, amt: take})
				left = left.Sub(take)
			}
		}
	}
	old := func() {
		pl.old = decimal.Min(s.old, left)
		left = left.Sub(pl.old)
	}
	if mode == "old" {
		old()
		bills()
	} else {
		bills()
		old()
	}
	pl.rest = left
	return pl
}

// pay books a payment (or a discount: bills credited, no advance) (SUP-4, SUP-6).
func (s *pSup) pay(pl plan, amount decimal.Decimal, discount bool) *pPay {
	p := &pPay{amount: amount, discount: discount, allocs: pl.allocs}
	for _, a := range pl.allocs {
		if discount {
			a.bill.credited = a.bill.credited.Add(a.amt)
		} else {
			a.bill.paid = a.bill.paid.Add(a.amt)
		}
	}
	s.old, s.bal = s.old.Sub(pl.old), s.bal.Sub(pl.old)
	s.addBal(amount.Sub(pl.old).Neg())
	s.pays = append(s.pays, p)
	return p
}

// sendBack books a debit note of value: off the balance, its own bill first
// (when it has something left), then the oldest; the rest is their credit (PUR-9).
func (s *pSup) sendBack(own *pBill, value decimal.Decimal) {
	r := &pRet{total: value}
	s.rets = append(s.rets, r)
	if !value.IsPositive() {
		return
	}
	s.addBal(value.Neg())
	order := s.open()
	for i, b := range order {
		if b == own {
			order = append(append([]*pBill{b}, order[:i]...), order[i+1:]...)
			break
		}
	}
	left := value
	for _, b := range order {
		if take := decimal.Min(b.left(), left); take.IsPositive() {
			b.credited = b.credited.Add(take)
			r.allocated = r.allocated.Add(take)
			left = left.Sub(take)
		}
	}
}

// bounce takes a cheque's amount back: owed again, its payment's bills
// reopened newest first, the rest off what was still an advance (CHQ-3, CHQ-4).
func (s *pSup) bounce(p *pPay, amount decimal.Decimal) {
	s.addBal(amount)
	left := amount
	for i := len(p.allocs) - 1; i >= 0 && left.IsPositive(); i-- {
		a := p.allocs[i]
		if take := decimal.Min(left, a.amt.Sub(a.reopened)); take.IsPositive() {
			a.bill.paid = decimal.Max(a.bill.paid.Sub(take), zero)
			a.reopened = a.reopened.Add(take)
			left = left.Sub(take)
		}
	}
	p.back = p.back.Add(amount)
}

// adjustOpening sets the old debt: the balance moves by the change; a credit
// they hold pays the new old debt down (SUP-2).
func (s *pSup) adjustOpening(to decimal.Decimal) {
	s.bal = s.bal.Add(to.Sub(s.old))
	s.old = decimal.Min(to, decimal.Max(s.bal, zero))
}

// lineNet is a delivery line's net (PUR-1): paid units × cost less its discount,
// fixed per unit or a percent of the line, within the line.
func lineNet(q, cost, disc decimal.Decimal, percent bool) decimal.Decimal {
	gross := q.Mul(cost).Round(2)
	off := disc.Mul(q).Round(2)
	if percent {
		off = gross.Mul(disc).Div(decimal.NewFromInt(100)).Round(2)
	}
	return gross.Sub(decimal.Min(decimal.Max(off, zero), gross))
}

// billTotal takes the bill discount (fixed or a percent) off the lines, within them.
func billTotal(sub, disc decimal.Decimal, percent bool) decimal.Decimal {
	off := disc.Round(2)
	if percent {
		off = sub.Mul(disc).Div(decimal.NewFromInt(100)).Round(2)
	}
	return sub.Sub(decimal.Min(decimal.Max(off, zero), sub))
}
