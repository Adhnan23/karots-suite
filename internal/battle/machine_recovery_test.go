package battle

import (
	"fmt"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Losses in the purchasing machine (docs/rules/recovery.md): a supplier's lot
// written off, then settled with them — a replacement (sometimes with an extra
// charge), a credit, or nothing — and sometimes undone. The books follow REC-2,
// REC-3, REC-8 and PUR-8 (a credit's waiting part settles the next bills).

// pLoss is one write-off of a supplier's lot.
type pLoss struct {
	mv      int64
	sup     *pSup
	product int64
	qty     decimal.Decimal
	worth   decimal.Decimal
	rec     *pRec // nil while it waits
}

// pRec is a recovery as the books see it.
type pRec struct {
	id       int64
	outcome  string
	loss     *pLoss
	extra    decimal.Decimal // the extra charge, to the cent
	amount   decimal.Decimal // a credit's amount
	bills    map[*pBill]decimal.Decimal
	old      decimal.Decimal // the old debt it paid
	waited   decimal.Decimal // what waited for the next bills at first
	waiting  decimal.Decimal // what still waits
	moveSeen int             // the product's movements once it came in (REC-8)
	onBooks  bool            // it stands on the supplier's books (it named them)
}

// lose writes off part of a lot a supplier delivered.
func (m *purchasingMachine) lose(t *rapid.T) {
	sp := m.sup(t)
	var lots []*pLot
	for _, b := range sp.bills {
		for _, l := range b.lots {
			if l.left.GreaterThanOrEqual(decimal.NewFromInt(1)) {
				lots = append(lots, l)
			}
		}
	}
	if len(lots) == 0 {
		t.Skip("nothing to write off")
	}
	l := rapid.SampledFrom(lots).Draw(t, "lot")
	q := decimal.NewFromInt(int64(rapid.IntRange(1, int(l.left.IntPart())).Draw(t, "lost")))
	var r struct{ LossID, SupplierID int64 }
	m.s.must(t, m.s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": l.product, "quantity": q.String(), "reason": "damage", "batchId": l.id}, &r))
	if r.SupplierID != sp.id {
		t.Fatalf("REC-3 a write-off of %s's lot offers supplier %d", sp.name, r.SupplierID)
	}
	l.left = l.left.Sub(q)
	m.moves[l.product]++
	var worth decimal.Decimal
	m.s.must(t, m.s.db.Get(&worth, `SELECT cost FROM stock_movements WHERE id = $1`, r.LossID))
	m.losses = append(m.losses, &pLoss{mv: r.LossID, sup: sp, product: l.product, qty: q, worth: worth})
}

// recover settles a waiting loss with its supplier.
func (m *purchasingMachine) recover(t *rapid.T) {
	var waiting []*pLoss
	for _, l := range m.losses {
		if l.rec == nil {
			waiting = append(waiting, l)
		}
	}
	if len(waiting) == 0 {
		t.Skip("no loss waiting")
	}
	l := rapid.SampledFrom(waiting).Draw(t, "loss")
	sp := l.sup
	rec := &pRec{loss: l, outcome: rapid.SampledFrom([]string{"replacement", "paid", "written_off"}).Draw(t, "outcome")}
	body := map[string]any{"sourceType": "damage", "sourceId": l.mv, "outcome": rec.outcome}
	ok := true
	switch rec.outcome {
	case "replacement":
		body["expiry"] = "2031-01-01"
		if rapid.Bool().Draw(t, "extra") {
			e := cents(t, 1, 5000, "extra each")
			body["extraPerUnit"], body["supplierId"] = e.String(), sp.id
			rec.extra = e.Mul(l.qty).Round(2)
			ok = sp.active // REC-3: no extra charge for a disabled supplier
		}
	case "paid":
		rec.amount = cents(t, 1, 300000, "credit")
		body["amount"], body["supplierId"] = rec.amount.String(), sp.id
	}
	if !m.try(t, ok, "REC-3", rec.outcome+" for "+sp.name, func() error { return m.s.admin.Do("POST", "/api/v1/admin/recovery", body, nil) }) {
		return
	}
	m.s.must(t, m.s.db.Get(&rec.id, `SELECT id FROM loss_recoveries WHERE source_type = 'damage' AND source_id = $1`, l.mv))
	l.rec = rec
	switch rec.outcome {
	case "replacement":
		m.moves[l.product]++
		rec.moveSeen = m.moves[l.product]
		if rec.extra.IsPositive() {
			sp.addBal(rec.extra)
			rec.onBooks = true
		}
	case "paid":
		// Off the balance (the old debt with it), then on open bills oldest first;
		// what's beyond all they're owed waits for the next bills (REC-3).
		oldBefore := sp.old
		sp.addBal(rec.amount.Neg())
		rec.old = oldBefore.Sub(sp.old)
		rec.bills = map[*pBill]decimal.Decimal{}
		left := rec.amount
		for _, b := range sp.open() {
			if take := decimal.Min(b.left(), left); take.IsPositive() {
				b.credited = b.credited.Add(take)
				rec.bills[b] = take
				left = left.Sub(take)
			}
		}
		rec.waited = decimal.Max(decimal.Min(left, sp.bal.Neg()), zero)
		rec.waiting = rec.waited
		sp.creds = append(sp.creds, rec)
		rec.onBooks = true
	}
	if rec.onBooks || rec.outcome == "paid" {
		sp.events = append(sp.events, rec)
	}
}

// undo takes a recovery back; the books say whether it may (REC-8).
func (m *purchasingMachine) undo(t *rapid.T) {
	var done []*pLoss
	for _, l := range m.losses {
		if l.rec != nil {
			done = append(done, l)
		}
	}
	if len(done) == 0 {
		t.Skip("nothing recovered")
	}
	l := rapid.SampledFrom(done).Draw(t, "recovered loss")
	rec, sp := l.rec, l.sup
	last := len(sp.events) > 0 && sp.events[len(sp.events)-1] == any(rec)
	ok := true
	switch rec.outcome {
	case "replacement":
		ok = m.moves[l.product] == rec.moveSeen && (!rec.onBooks || last)
	case "paid":
		back := sp.old.Add(rec.old)
		ok = last && rec.waiting.Equal(rec.waited) && !back.GreaterThan(sp.opening) && !back.GreaterThan(decimal.Max(sp.bal.Add(rec.amount), zero))
	}
	if !m.try(t, ok, "REC-8", "undo "+rec.outcome+" of "+sp.name, func() error {
		return m.s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/recovery/%d", rec.id), nil, nil)
	}) {
		return
	}
	l.rec = nil
	for i, e := range sp.events {
		if e == any(rec) {
			sp.events = append(sp.events[:i], sp.events[i+1:]...)
			break
		}
	}
	switch rec.outcome {
	case "replacement":
		m.moves[l.product]++
		if rec.extra.IsPositive() {
			sp.addBal(rec.extra.Neg())
		}
	case "paid":
		for b, amt := range rec.bills {
			b.credited = b.credited.Sub(amt)
		}
		sp.bal, sp.old = sp.bal.Add(rec.amount), sp.old.Add(rec.old)
		for i, c := range sp.creds {
			if c == rec {
				sp.creds = append(sp.creds[:i], sp.creds[i+1:]...)
				break
			}
		}
	}
}
