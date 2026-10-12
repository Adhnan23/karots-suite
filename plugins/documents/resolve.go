package documents

import (
	"karots-suite/internal/features/recipes"

	"github.com/shopspring/decimal"
)

// Price is one cell of a service's grid: a size (nil = any size), colour and
// side, from MinQty copies up.
type Price struct {
	ID         int64           `db:"id"`
	ServiceID  int64           `db:"service_id"`
	Size       *string         `db:"size"`
	Color      bool            `db:"color"`
	DoubleSide bool            `db:"double_side"`
	MinQty     int             `db:"min_qty"`
	UnitPrice  decimal.Decimal `db:"unit_price"`
}

// Paper is what one SHEET of a size uses (nil = any size): whole units, so a
// one-copy job still takes a whole sheet.
type Paper struct {
	ID         int64           `db:"id"`
	ServiceID  int64           `db:"service_id"`
	Size       *string         `db:"size"`
	ProductID  int64           `db:"product_id"`
	QtyPerUnit decimal.Decimal `db:"qty_per_unit"`
	Name       string          `db:"product_name"`
	CostPrice  decimal.Decimal `db:"cost_price"`
}

// JobOptions is what the cashier picked for a metered job.
type JobOptions struct {
	Size   string
	Color  bool
	Double bool
	Copies int
}

func sizeIs(p *string, size string) bool { return p != nil && *p == size }

// PickPrice is karots-pos ResolveUnitPrice: among the cells for this colour and
// side whose MinQty ≤ copies, a row for the size beats an any-size row, then the
// greatest MinQty (the best tier reached) wins.
func PickPrice(rows []Price, o JobOptions) (Price, bool) {
	var best Price
	found := false
	for _, r := range rows {
		if r.Color != o.Color || r.DoubleSide != o.Double || r.MinQty > o.Copies {
			continue
		}
		if r.Size != nil && *r.Size != o.Size {
			continue
		}
		if !found || better(r, best, o.Size) {
			best, found = r, true
		}
	}
	return best, found
}

func better(a, b Price, size string) bool {
	as, bs := sizeIs(a.Size, size), sizeIs(b.Size, size)
	if as != bs {
		return as
	}
	return a.MinQty > b.MinQty
}

// Sheets a job prints on: two impressions to a sheet when double-sided.
func Sheets(o JobOptions) int {
	if o.Double {
		return (o.Copies + 1) / 2
	}
	return o.Copies
}

// PaperFor is the paper of the size when it has its own, else the any-size paper.
func PaperFor(all []Paper, size string) []Paper {
	var own, anySize []Paper
	for _, p := range all {
		switch {
		case sizeIs(p.Size, size):
			own = append(own, p)
		case p.Size == nil:
			anySize = append(anySize, p)
		}
	}
	if len(own) > 0 {
		return own
	}
	return anySize
}

// Components is everything a job consumes and its estimated cost: the paper per
// sheet (whole units), then the service's core recipe per copy (toner, ink —
// fractional). A product already counted as paper is not taken again from the
// recipe, or the same sheets would leave twice.
func Components(paper []Paper, recipe []recipes.Component, o JobOptions) ([]recipes.Consumption, decimal.Decimal) {
	sheets := decimal.NewFromInt(int64(Sheets(o)))
	out := []recipes.Consumption{}
	cost := decimal.Zero
	seen := map[int64]bool{}
	for _, p := range paper {
		q := p.QtyPerUnit.Mul(sheets).Ceil()
		if !q.IsPositive() {
			continue
		}
		seen[p.ProductID] = true
		out = append(out, recipes.Consumption{ProductID: p.ProductID, Qty: q})
		cost = cost.Add(q.Mul(p.CostPrice))
	}
	copies := decimal.NewFromInt(int64(o.Copies))
	for _, c := range recipe {
		if seen[c.ComponentProductID] {
			continue
		}
		if q := c.Consumed(copies); q.IsPositive() {
			out = append(out, recipes.Consumption{ProductID: c.ComponentProductID, Qty: q})
			cost = cost.Add(q.Mul(c.CostPrice))
		}
	}
	return out, cost.Round(2)
}
