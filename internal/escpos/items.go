package escpos

import (
	"bytes"
	"strconv"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/money"

	"github.com/shopspring/decimal"
)

// printItems prints the sale's lines and returns the item discounts. Lines the
// till added together (a line group) print once, as one block, where the
// group's first line stood.
func printItems(b *bytes.Buffer, d sales.Detail, w int, opts Options) decimal.Decimal {
	itemDisc := decimal.Zero
	done := map[int]bool{}
	for _, it := range d.Items {
		if it.LineGroup == nil || it.LineGroupLabel == nil || it.LineGroupSets == nil {
			itemDisc = itemDisc.Add(printItem(b, it, d.Sale.CreatedAt, w, opts))
			continue
		}
		if done[*it.LineGroup] {
			continue
		}
		done[*it.LineGroup] = true
		var members []sales.SaleItem
		for _, m := range d.Items {
			if m.LineGroup != nil && *m.LineGroup == *it.LineGroup {
				members = append(members, m)
			}
		}
		if it.LineGroupItemised {
			itemDisc = itemDisc.Add(printItemised(b, *it.LineGroupLabel, members, d.Sale.CreatedAt, w, opts))
			continue
		}
		itemDisc = itemDisc.Add(printGroup(b, *it.LineGroupLabel, *it.LineGroupSets, members, d.Sale.CreatedAt, w, opts))
	}
	return itemDisc
}

// printItem is one plain line; it returns its discount.
func printItem(b *bytes.Buffer, it sales.SaleItem, soldAt time.Time, w int, opts Options) decimal.Decimal {
	for _, ln := range wrap(ascii(it.Label()), w) {
		line(b, ln)
	}
	if it.Subtotal.IsNegative() { // a payout: money handed to the customer
		line(b, leftRight("  Given", "-"+money.Display(it.Subtotal.Neg()), w))
		extras(b, it, soldAt, w, opts, "  ")
		return decimal.Zero
	}
	qty := money.Display(it.Quantity) + " " + ascii(it.UnitAbbr) + " x " + money.Display(it.UnitPrice)
	// The qty line always shows the GROSS (qty × unit price); a discount then
	// shows the discounted unit price (was -> now), its own reduction, and the
	// resulting line total, so the numbers add up on paper (a printer can't
	// strike-through like the webview does).
	if it.PackLabel != nil {
		// Sold partly by the pack: the split is what adds up, not one unit price.
		qty = money.Display(it.Quantity) + " " + ascii(it.UnitAbbr) + " (" + ascii(*it.PackLabel) + ")"
	} else if it.Discount.IsPositive() {
		qty += " -> " + money.Display(it.NetUnitPrice())
	}
	line(b, leftRight("  "+qty, money.Display(it.Gross()), w))
	if it.Discount.IsPositive() {
		line(b, leftRight("  You saved"+discSuffix(it.DiscountType, it.DiscountValue), "-"+money.Display(it.Discount), w))
		line(b, leftRight("  Line total", money.Display(it.Subtotal), w))
	}
	extras(b, it, soldAt, w, opts, "  ")
	return it.Discount
}

// printGroup is a line group as one block, in a discounted line's shape:
// sets x a set's full price -> its price, You saved, Line total, then the
// members without prices. It returns the group's discount.
func printGroup(b *bytes.Buffer, label string, sets decimal.Decimal, members []sales.SaleItem, soldAt time.Time, w int, opts Options) decimal.Decimal {
	gross, disc, net := decimal.Zero, decimal.Zero, decimal.Zero
	for _, m := range members {
		gross, disc, net = gross.Add(m.Gross()), disc.Add(m.Discount), net.Add(m.Subtotal)
	}
	for _, ln := range wrap(ascii(label), w) {
		line(b, ln)
	}
	qty := sets.String() + " x " + money.Display(gross.Div(sets).Round(2))
	if disc.IsPositive() {
		qty += " -> " + money.Display(net.Div(sets).Round(2))
	}
	line(b, leftRight("  "+qty, money.Display(gross), w))
	if disc.IsPositive() {
		line(b, leftRight("  You saved", "-"+money.Display(disc), w))
		line(b, leftRight("  Line total", money.Display(net), w))
	}
	for _, m := range members {
		for i, ln := range wrap(ascii(m.Label())+" x"+m.Quantity.String(), w-5) {
			line(b, map[bool]string{true: "   - ", false: "     "}[i == 0]+ln)
		}
		extras(b, m, soldAt, w, opts, "     ")
	}
	return disc
}

// printItemised is an itemised group (a job and its add-ons): the label with the
// group's total, then each member with its own price (net of its discount, the
// saving under it). It returns the group's discount.
func printItemised(b *bytes.Buffer, label string, members []sales.SaleItem, soldAt time.Time, w int, opts Options) decimal.Decimal {
	disc, net := decimal.Zero, decimal.Zero
	for _, m := range members {
		disc, net = disc.Add(m.Discount), net.Add(m.Subtotal)
	}
	wrapRight(b, "", "", ascii(label), money.Display(net), w)
	for _, m := range members {
		wrapRight(b, "   - ", "     ", ascii(m.Label())+" x"+m.Quantity.String(), money.Display(m.Subtotal), w)
		if m.Discount.IsPositive() {
			line(b, leftRight("     You saved", "-"+money.Display(m.Discount), w))
		}
		extras(b, m, soldAt, w, opts, "     ")
	}
	return disc
}

// wrapRight wraps text and puts amount on its last line, flush right.
func wrapRight(b *bytes.Buffer, first, rest, text, amount string, w int) {
	lines := wrap(text, w-len(first)-len(amount)-1)
	for i, ln := range lines {
		ln = map[bool]string{true: first, false: rest}[i == 0] + ln
		if i == len(lines)-1 {
			ln = leftRight(ln, amount, w)
		}
		line(b, ln)
	}
}

// extras are a line's warranty and serial numbers.
func extras(b *bytes.Buffer, it sales.SaleItem, soldAt time.Time, w int, opts Options, indent string) {
	if until := it.WarrantyUntil(soldAt); !until.IsZero() {
		warr := strconv.Itoa(it.WarrantyMonths) + "-month warranty until " + datetime.Date(until)
		for _, ln := range wrap(indent+warr, w) {
			line(b, ln)
		}
	}
	for _, sn := range opts.Serials[it.ProductID] {
		for _, ln := range wrap(ascii(sn), w) {
			line(b, indent+ln)
		}
	}
}
