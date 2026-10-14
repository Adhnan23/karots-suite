package sales

import (
	"sort"
	"strings"

	"karots-suite/internal/features/products"

	"github.com/shopspring/decimal"
)

// PackPrice is what pricing needs of a pack (products.Pack, or a test row).
type PackPrice struct {
	Name      string
	Size      decimal.Decimal
	Selling   decimal.Decimal // 0 = size × unit
	Wholesale decimal.Decimal // 0 = Selling
}

func packPrices(ks []products.Pack) []PackPrice {
	out := make([]PackPrice, len(ks))
	for i, k := range ks {
		out[i] = PackPrice{Name: k.Name, Size: k.Size, Selling: k.SellingPrice, Wholesale: k.WholesalePrice}
	}
	return out
}

// PackGross prices qty units when the product is also sold by the pack: every
// full pack (largest first) rings at the pack's price when that is cheaper than
// its units, the rest at the unit price. label is the split as handed over
// ("1 Card + 2"), "" when no pack fits. Mirrored by web cart/packs.ts — both
// read packs.fixture.json.
func PackGross(qty, unit decimal.Decimal, packs []PackPrice, wholesale bool) (decimal.Decimal, string) {
	ks := append([]PackPrice(nil), packs...)
	sort.SliceStable(ks, func(a, b int) bool { return ks[a].Size.GreaterThan(ks[b].Size) })
	gross := decimal.Zero
	rest := qty
	var parts []string
	for _, k := range ks {
		if !k.Size.IsPositive() {
			continue
		}
		n := rest.Div(k.Size).Floor()
		if !n.IsPositive() {
			continue
		}
		asUnits := k.Size.Mul(unit)
		price := k.Selling
		if wholesale && k.Wholesale.IsPositive() {
			price = k.Wholesale
		}
		if !price.IsPositive() || price.GreaterThan(asUnits) {
			price = asUnits
		}
		gross = gross.Add(n.Mul(price))
		rest = rest.Sub(n.Mul(k.Size))
		parts = append(parts, n.String()+" "+packPlural(k.Name, n))
	}
	gross = gross.Add(rest.Mul(unit)).Round(2)
	if len(parts) == 0 {
		return gross, ""
	}
	if rest.IsPositive() {
		parts = append(parts, rest.String())
	}
	return gross, strings.Join(parts, " + ")
}

// packPlural: "1 Card", "2 Cards", "2 Boxes", "2 Sachets".
func packPlural(name string, n decimal.Decimal) string {
	if n.Equal(decimal.NewFromInt(1)) {
		return name
	}
	low := strings.ToLower(name)
	switch {
	case strings.HasSuffix(low, "s"):
		return name
	case strings.HasSuffix(low, "x"), strings.HasSuffix(low, "ch"), strings.HasSuffix(low, "sh"):
		return name + "es"
	}
	return name + "s"
}
