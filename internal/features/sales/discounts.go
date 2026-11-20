package sales

import "github.com/shopspring/decimal"

// Discounts and tax on a sale: the maths Create prices every line and the bill with.

var hundred = decimal.NewFromInt(100)

// normDiscountType defaults a blank discount type to "fixed".
func normDiscountType(t string) string {
	if t == "percent" {
		return "percent"
	}
	return "fixed"
}

// resolveBillDiscount turns the bill-level discount (a flat fixed amount, or a
// percentage of base) into a concrete amount, clamped to [0, base].
func resolveBillDiscount(dtype string, value, base decimal.Decimal) decimal.Decimal {
	amt := value
	if dtype == "percent" {
		amt = base.Mul(value).Div(hundred).Round(2)
	}
	return clampDiscount(amt, base)
}

// resolveItemDiscount turns a per-item discount into a line amount. A fixed
// value is PER UNIT and multiplies by quantity (Rs 5 off × 3 = Rs 15); a percent
// is taken off the line gross (equivalent to % of the unit price). Clamped to
// [0, lineGross].
func resolveItemDiscount(dtype string, value, lineGross, qty decimal.Decimal) decimal.Decimal {
	amt := value.Mul(qty).Round(2)
	if dtype == "percent" {
		amt = lineGross.Mul(value).Div(hundred).Round(2)
	}
	return clampDiscount(amt, lineGross)
}

func clampDiscount(amt, base decimal.Decimal) decimal.Decimal {
	if amt.IsNegative() {
		return decimal.Zero
	}
	if amt.GreaterThan(base) {
		return base
	}
	return amt
}

// taxAfterBill: tax is charged on what the customer pays for each line — its
// net less its share of the bill discount (shared to the cent, shareOut) —
// each line's tax rounded to the cent (SALE-35). With no bill discount it is
// the lines' own tax, unchanged.
func taxAfterBill(lines []SaleItem, rates []decimal.Decimal, bill, lineTax decimal.Decimal) decimal.Decimal {
	if !bill.IsPositive() || !lineTax.IsPositive() {
		return lineTax
	}
	nets := make([]decimal.Decimal, len(lines))
	for i, l := range lines {
		nets[i] = l.Subtotal
	}
	shares := shareOut(bill, nets)
	tax := decimal.Zero
	for i, l := range lines {
		if l.Subtotal.IsPositive() && rates[i].IsPositive() {
			tax = tax.Add(l.Subtotal.Sub(shares[i]).Mul(rates[i]).Div(hundred).Round(2))
		}
	}
	return tax
}
