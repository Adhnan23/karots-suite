package sales

import (
	"sort"

	"github.com/shopspring/decimal"
)

// PaidShares is what the customer paid for each line (by sale item id): the
// goods' part of the total (total − payouts, so after the bill discount and
// with tax) shared over the goods' lines in proportion to their net. The
// shares add up to that part exactly, to the cent (DISC-5, RET-5): each line
// takes its exact share rounded down, and the cents left over go one each to
// the lines with the largest remainders (ties: the earlier line). Rounding each
// line on its own refunded more than was paid (3 × Rs 100, Rs 10 off: 96.67 × 3
// = 290.01 for 290). A payout line's share is its own (negative) amount.
func PaidShares(total decimal.Decimal, items []SaleItem) map[int64]decimal.Decimal {
	out := make(map[int64]decimal.Decimal, len(items))
	goods, base := total, decimal.Zero
	for _, it := range items {
		if it.Subtotal.IsNegative() {
			out[it.ID] = it.Subtotal
			goods = goods.Sub(it.Subtotal)
		} else if it.Subtotal.IsPositive() {
			base = base.Add(it.Subtotal)
		}
	}
	type part struct {
		id    int64
		order int
		rem   decimal.Decimal
	}
	var parts []part
	left := goods.Shift(2).Round(0) // cents still to give out
	for i, it := range items {
		if it.Subtotal.IsNegative() {
			continue
		}
		if !it.Subtotal.IsPositive() || base.IsZero() {
			out[it.ID] = decimal.Zero
			continue
		}
		exact := it.Subtotal.Mul(goods).Shift(2).Div(base) // in cents
		cents := exact.Floor()
		out[it.ID] = cents.Shift(-2)
		left = left.Sub(cents)
		parts = append(parts, part{it.ID, i, exact.Sub(cents)})
	}
	sort.SliceStable(parts, func(a, b int) bool {
		if c := parts[a].rem.Cmp(parts[b].rem); c != 0 {
			return c > 0
		}
		return parts[a].order < parts[b].order
	})
	for i := 0; left.IsPositive() && len(parts) > 0; i = (i + 1) % len(parts) {
		out[parts[i].id] = out[parts[i].id].Add(decimal.New(1, -2))
		left = left.Sub(decimal.NewFromInt(1))
	}
	return out
}
