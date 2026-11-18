package sales

import (
	"sort"

	"github.com/shopspring/decimal"
)

// shareOut splits amount over weights in proportion, to the cent, so the
// shares add up to amount exactly: each takes its exact share rounded down,
// and the cents left over go one each to the largest remainders (ties: the
// earlier). A weight ≤ 0 gets nothing. Used for what each line was paid
// (PaidShares) and how a bill discount falls on each line's tax (SALE-35).
func shareOut(amount decimal.Decimal, weights []decimal.Decimal) []decimal.Decimal {
	out := make([]decimal.Decimal, len(weights))
	base := decimal.Zero
	for _, w := range weights {
		if w.IsPositive() {
			base = base.Add(w)
		}
	}
	if base.IsZero() {
		return out
	}
	type part struct {
		i   int
		rem decimal.Decimal
	}
	var parts []part
	left := amount.Shift(2).Round(0)
	for i, w := range weights {
		if !w.IsPositive() {
			continue
		}
		exact := w.Mul(amount).Shift(2).Div(base)
		c := exact.Floor()
		out[i] = c.Shift(-2)
		left = left.Sub(c)
		parts = append(parts, part{i, exact.Sub(c)})
	}
	sort.SliceStable(parts, func(a, b int) bool { return parts[a].rem.GreaterThan(parts[b].rem) })
	for k := 0; left.IsPositive(); k = (k + 1) % len(parts) {
		out[parts[k].i] = out[parts[k].i].Add(decimal.New(1, -2))
		left = left.Sub(decimal.NewFromInt(1))
	}
	return out
}

// PaidShares is what the customer paid for each line (by sale item id): the
// goods' part of the total (total − payouts, so after the bill discount and
// with tax) shared over the goods' lines by their net, to the cent (DISC-5,
// RET-5). Rounding each line on its own refunded more than was paid (3 × Rs
// 100, Rs 10 off: 96.67 × 3 = 290.01 for 290). A payout line's share is its
// own (negative) amount.
func PaidShares(total decimal.Decimal, items []SaleItem) map[int64]decimal.Decimal {
	goods := total
	nets := make([]decimal.Decimal, len(items))
	for i, it := range items {
		nets[i] = it.Subtotal
		if it.Subtotal.IsNegative() {
			goods = goods.Sub(it.Subtotal)
		}
	}
	shares := shareOut(goods, nets)
	out := make(map[int64]decimal.Decimal, len(items))
	for i, it := range items {
		out[it.ID] = shares[i]
		if it.Subtotal.IsNegative() {
			out[it.ID] = it.Subtotal
		}
	}
	return out
}
