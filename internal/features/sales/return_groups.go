package sales

import (
	"fmt"

	"github.com/shopspring/decimal"

	"karots-suite/internal/apperr"
	"karots-suite/internal/money"
)

// checkWholeSets refuses a return that breaks up a line group (a combo, a job
// and its add-ons): a group comes back only whole — every one of its lines, in
// the same whole number of sets (2 of 3 sets = each line's qty × 2⁄3) (RET-13,
// user 2026-11-18). Lines outside any group are free.
func checkWholeSets(all []SaleItem, lines []ReturnLineInput) error {
	asked := map[int64]decimal.Decimal{}
	for _, ln := range lines {
		q, err := money.Parse(ln.Quantity)
		if err != nil {
			return apperr.Validation("return quantity must be greater than zero")
		}
		asked[ln.SaleItemID] = asked[ln.SaleItemID].Add(q)
	}
	type grp struct {
		label string
		sets  *decimal.Decimal // the same for every line, or the return is refused
		odd   bool
	}
	groups := map[int]*grp{}
	for _, it := range all {
		if it.LineGroup == nil || it.LineGroupSets == nil || !it.Quantity.IsPositive() {
			continue
		}
		g := groups[*it.LineGroup]
		if g == nil {
			g = &grp{label: "this group"}
			if it.LineGroupLabel != nil && *it.LineGroupLabel != "" {
				g.label = *it.LineGroupLabel
			}
			groups[*it.LineGroup] = g
		}
		k := asked[it.ID].Mul(*it.LineGroupSets).Div(it.Quantity) // sets this line brings back
		switch {
		case g.sets == nil:
			g.sets = &k
		case !g.sets.Equal(k):
			g.odd = true
		}
	}
	for _, g := range groups {
		if g.sets == nil || g.sets.IsZero() && !g.odd {
			continue // not in this return
		}
		if g.odd || !g.sets.Equal(g.sets.Floor()) {
			return apperr.Validation(fmt.Sprintf("%s comes back whole: every item in it, in whole sets", g.label))
		}
	}
	return nil
}
