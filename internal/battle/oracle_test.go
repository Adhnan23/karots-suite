package battle

import "github.com/shopspring/decimal"

// The battle's own reading of the rule book's pricing (docs/rules/sales.md, "How
// a sale is valued"), written apart from internal/features/sales so the two
// can disagree: the server's total must equal this one (SALE-1, DISC-1…4).

var hundred = decimal.NewFromInt(100)

// item is one line the machine rings.
type item struct {
	kind     string          // a fixture key: plain, odd, cheap, loose, pack
	qty      decimal.Decimal // in the product's unit
	discType string          // "", "fixed" (per unit) or "percent"
	disc     decimal.Decimal
	group    int // 0 = none; lines of a group share sets
	sets     int
}

// unitPrice of each fixture (shop_test.go fixtures).
var unitPrice = map[string]decimal.Decimal{
	"plain": decimal.RequireFromString("100"), "odd": decimal.RequireFromString("33.33"),
	"cheap": decimal.RequireFromString("0.99"), "loose": decimal.RequireFromString("245.50"),
	"pack": decimal.RequireFromString("12.50"),
}

// gross: qty × unit, the pack's full Cards (12 for Rs 140) at the pack price —
// not inside a group, where packs don't apply.
func (it item) gross() decimal.Decimal {
	u := unitPrice[it.kind]
	if it.kind == "pack" && it.group == 0 {
		cards := it.qty.Div(decimal.NewFromInt(12)).Floor()
		rest := it.qty.Sub(cards.Mul(decimal.NewFromInt(12)))
		return cards.Mul(decimal.NewFromInt(140)).Add(rest.Mul(u)).Round(2)
	}
	return it.qty.Mul(u).Round(2)
}

func clamp(v, hi decimal.Decimal) decimal.Decimal {
	return decimal.Min(decimal.Max(v, decimal.Zero), hi)
}

func (it item) net() decimal.Decimal {
	g := it.gross()
	switch it.discType {
	case "fixed":
		return g.Sub(clamp(it.disc.Mul(it.qty).Round(2), g))
	case "percent":
		return g.Sub(clamp(g.Mul(it.disc).Div(hundred).Round(2), g))
	}
	return g
}

// total of a sale: the lines' net less the bill discount (no tax: the battle
// shop isn't tax-registered).
func total(items []item, billType string, bill decimal.Decimal) decimal.Decimal {
	net := decimal.Zero
	for _, it := range items {
		net = net.Add(it.net())
	}
	off := bill
	if billType == "percent" {
		off = net.Mul(bill).Div(hundred).Round(2)
	}
	return net.Sub(clamp(off, net)).Round(2)
}
