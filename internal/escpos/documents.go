// Package escpos renders a sale as an ESC/POS byte stream for a thermal receipt
// printer (e.g. Xprinter POS-80). It uses the printer's built-in font rather
// than raster graphics, so printing is fast and immune to the PDF/codepage
// garbage that browser printing to a raw CUPS queue produces. The width is
// driven by the shop's receipt_width setting (80mm = 48 cols, 58mm = 32 cols).
package escpos

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/money"

	"github.com/shopspring/decimal"
)

// Document builds the ESC/POS byte stream for a completed sale.
func Document(d sales.Detail, cfg settings.Settings, opts Options) []byte {
	w := columns(cfg.ReceiptWidth)
	sym := cfg.CurrencySymbol
	if sym == "" {
		sym = "Rs."
	}

	var b bytes.Buffer
	Init(&b)
	Header(&b, cfg, opts)

	// --- Ticket strip: receipt no + time, who served, for whom ---
	b.Write([]byte{esc, 'a', 0})        // left
	if StyleFor(cfg).HeaderRule == "" { // a ruled header already ends in a rule
		divider(&b, cfg, w)
	}
	b.Write([]byte{esc, 'E', 1})
	line(&b, leftRight(d.Sale.ReceiptNo, datetime.DateTime(d.Sale.CreatedAt), w))
	b.Write([]byte{esc, 'E', 0})
	line(&b, "Served by "+ascii(d.Sale.CashierName))
	if d.Sale.CustomerName != nil && *d.Sale.CustomerName != "" {
		line(&b, "Customer: "+ascii(*d.Sale.CustomerName))
	}
	divider(&b, cfg, w)

	// --- Items: ITEM / QTY / AMOUNT ---
	tableHead(&b, w)
	_, qw, _ := itemCols(w)
	itemDisc, units := decimal.Zero, decimal.Zero
	for _, it := range d.Items {
		units = units.Add(it.Quantity)
		qty := qtyText(it.Quantity)
		if u := ascii(it.UnitAbbr); u != "" && u != "pcs" && u != "pc" {
			if len(qty)+len(u)+1 <= qw {
				qty += " " + u
			} else if len(qty)+len(u) <= qw {
				qty += u
			}
		}
		itemRow(&b, w, ascii(it.ProductName), qty, money.Display(it.Gross()))
		// The unit price under the name when it isn't obvious (more than one, or
		// discounted: was -> now), so the amounts add up on paper.
		if it.Discount.IsPositive() {
			itemDisc = itemDisc.Add(it.Discount)
			line(&b, "  @ "+money.Display(it.UnitPrice)+" -> "+money.Display(it.NetUnitPrice()))
			line(&b, leftRight("  Discount"+discSuffix(it.DiscountType, it.DiscountValue), "-"+money.Display(it.Discount), w))
		} else if !it.Quantity.Equal(decimal.NewFromInt(1)) {
			line(&b, "  @ "+money.Display(it.UnitPrice))
		}
		if until := it.WarrantyUntil(d.Sale.CreatedAt); !until.IsZero() {
			warr := strconv.Itoa(it.WarrantyMonths) + "-month warranty until " + datetime.Date(until)
			for _, ln := range wrap("  "+warr, w) {
				line(&b, ln)
			}
		}
		for _, sn := range opts.Serials[it.ProductID] {
			for _, ln := range wrap(ascii(sn), w) {
				line(&b, "  "+ln)
			}
		}
	}
	divider(&b, cfg, w)
	line(&b, plural(len(d.Items), "item")+", "+qtyText(units)+" units")

	// --- Totals, right-hand block ---
	// Sale.Discount holds item discounts + bill discount; split them so the
	// receipt mirrors the cashier screen (item discounts shown per line above).
	// Subtotal only when a discount or tax sits between it and the total.
	if d.Sale.Discount.IsPositive() || d.Sale.Tax.IsPositive() {
		sumRow(&b, w, "Subtotal", money.Format(sym, d.Sale.Subtotal))
	}
	if itemDisc.IsPositive() {
		sumRow(&b, w, "Item discounts", "-"+money.Format(sym, itemDisc))
	}
	if billDisc := d.Sale.Discount.Sub(itemDisc); billDisc.IsPositive() {
		sumRow(&b, w, "Bill discount"+discSuffix(d.Sale.DiscountType, d.Sale.DiscountValue), "-"+money.Format(sym, billDisc))
	}
	if d.Sale.Tax.IsPositive() {
		sumRow(&b, w, "Tax", money.Format(sym, d.Sale.Tax))
	}
	line(&b, "")
	bigLine(&b, cfg, "TOTAL", money.Format(sym, d.Sale.Total), w)

	// One line per tender (cash / card / online / …).
	gap := true
	for _, p := range d.Payments {
		if p.Method == "credit" { // shown as "On account" below
			continue
		}
		if gap {
			line(&b, "")
			gap = false
		}
		sumRow(&b, w, "Paid by "+strings.ToLower(ascii(p.Method)), money.Format(sym, p.Amount))
	}

	// A credit sale says what went on the account and where the account now
	// stands (owed, or the advance left) — one figure each, never the same
	// amount twice. A cash sale ends on the change.
	if cs, ok := d.Sale.Credit(opts.CustomerDue); ok {
		line(&b, "")
		if cs.FromAdvance.IsPositive() {
			sumRow(&b, w, "From advance", "-"+money.Format(sym, cs.FromAdvance))
		}
		if due := cs.Due.Sub(cs.FromAdvance); due.IsPositive() {
			sumRow(&b, w, "On account", money.Format(sym, due))
		}
		if cs.Balance.IsNegative() {
			bigLine(&b, cfg, "ADVANCE LEFT", money.Format(sym, cs.Balance.Neg()), w)
		} else {
			bigLine(&b, cfg, "BALANCE OWED", money.Format(sym, cs.Balance), w)
		}
	} else if d.Sale.ChangeGiven.IsPositive() {
		line(&b, "")
		bigLine(&b, cfg, "CHANGE", money.Format(sym, d.Sale.ChangeGiven), w)
	}
	// Rounding: cash the customer left behind (paid - change - total), booked to
	// their account (On account) or kept as a rounding gain.
	if kept := d.Sale.PaidAmount.Sub(d.Sale.ChangeGiven).Sub(d.Sale.Total); kept.IsPositive() {
		if d.Sale.RoundingToAccount.IsPositive() {
			sumRow(&b, w, "Kept on account", money.Format(sym, d.Sale.RoundingToAccount))
		}
		if cash := kept.Sub(d.Sale.RoundingToAccount); cash.IsPositive() {
			sumRow(&b, w, "Change kept", money.Format(sym, cash))
		}
	}
	if d.Sale.Discount.IsPositive() {
		line(&b, "")
		b.Write([]byte{esc, 'E', 1})
		line(&b, center("** You saved "+money.Format(sym, d.Sale.Discount)+" **", w))
		b.Write([]byte{esc, 'E', 0})
	}
	line(&b, "")
	receiptBarcode(&b, d.Sale.ReceiptNo)

	Footer(&b, cfg)
	return b.Bytes()
}

// TestDocument builds a short ESC/POS slip used to verify printer wiring from the
// settings page: it prints the shop name, a marker and a timestamp, then cuts —
// just enough to confirm bytes reach the printer at the configured width.
func TestDocument(cfg settings.Settings) []byte {
	w := columns(cfg.ReceiptWidth)

	var b bytes.Buffer
	b.Write([]byte{esc, '@'})    // initialize
	b.Write([]byte{esc, 't', 0}) // code page PC437 (Latin)

	b.Write([]byte{esc, 'a', 1}) // center
	b.Write([]byte{esc, 'E', 1}) // bold on
	b.Write([]byte{gs, '!', 0x11})
	name := cfg.ShopName
	if strings.TrimSpace(name) == "" {
		name = "POS"
	}
	line(&b, ascii(name))
	b.Write([]byte{gs, '!', 0x00})
	b.Write([]byte{esc, 'E', 0})
	line(&b, "")
	line(&b, "*** PRINTER TEST ***")
	line(&b, "")

	b.Write([]byte{esc, 'a', 0}) // left
	divider(&b, cfg, w)
	line(&b, leftRight("Width:", cfg.ReceiptWidth+"mm ("+strconv.Itoa(w)+" cols)", w))
	line(&b, leftRight("Printed:", datetime.DateTime(time.Now()), w))
	divider(&b, cfg, w)

	b.Write([]byte{esc, 'a', 1}) // center
	line(&b, "If you can read this,")
	line(&b, "your printer is set up.")

	b.Write([]byte{esc, 'd', feedBeforeCut})
	b.Write([]byte{gs, 'V', 1})
	return b.Bytes()
}

// ReturnDocument builds the ESC/POS byte stream for a refund slip — the proof a
// customer is handed after a return. It mirrors Document's header/footer but
// lists only the returned lines and the refund/credit split.
func ReturnDocument(rr sales.ReturnReceipt, cfg settings.Settings, opts Options) []byte {
	w := columns(cfg.ReceiptWidth)
	sym := cfg.CurrencySymbol
	if sym == "" {
		sym = "Rs."
	}

	var b bytes.Buffer
	Init(&b)
	Header(&b, cfg, opts)
	Title(&b, cfg, "REFUND", w)

	// --- Meta (left) ---
	b.Write([]byte{esc, 'a', 0})
	divider(&b, cfg, w)
	line(&b, leftRight("Orig. receipt:", rr.ReceiptNo, w))
	line(&b, leftRight("Date:", datetime.DateTime(rr.CreatedAt), w))
	if rr.CustomerName != nil && *rr.CustomerName != "" {
		line(&b, leftRight("Customer:", ascii(*rr.CustomerName), w))
	}
	if rr.Reason != nil && *rr.Reason != "" {
		for _, ln := range wrap("Reason: "+ascii(*rr.Reason), w) {
			line(&b, ln)
		}
	}
	divider(&b, cfg, w)

	// --- Returned items ---
	for _, it := range rr.Items {
		for _, ln := range wrap(ascii(it.ProductName), w) {
			line(&b, ln)
		}
		qty := money.Display(it.Quantity) + " " + ascii(it.UnitAbbr)
		line(&b, leftRight("  "+qty, money.Format(sym, it.Refund), w))
	}
	divider(&b, cfg, w)

	// --- Totals ---
	b.Write([]byte{esc, 'E', 1})
	line(&b, leftRight("CASH REFUND", money.Format(sym, rr.Refund), w))
	b.Write([]byte{esc, 'E', 0})
	if rr.CreditReduction.IsPositive() {
		line(&b, leftRight("Credit reduced", money.Format(sym, rr.CreditReduction), w))
	}
	if rr.RemainingBalance != nil {
		line(&b, leftRight("Balance due", money.Format(sym, *rr.RemainingBalance), w))
	}
	divider(&b, cfg, w)

	// --- Footer (centered) ---
	b.Write([]byte{esc, 'a', 1})
	line(&b, "Refund slip - please retain.")
	Footer(&b, cfg)

	return b.Bytes()
}

// WarrantySlip is the data for a printed warranty-replacement slip — the proof
// a customer is handed when a faulty unit is swapped for a new one.
type WarrantySlip struct {
	ProductName   string
	OldSerial     string
	NewSerial     string
	WarrantyUntil string // pre-formatted date (e.g. "2027-06-13")
	WarrantyLeft  string // pre-formatted remaining cover (e.g. "11 mo left"); optional
	CustomerName  string
}

// WarrantyDocument builds the ESC/POS byte stream for a replacement slip. It
// mirrors the receipt header/footer and lists the swapped serials.
func WarrantyDocument(s WarrantySlip, cfg settings.Settings, opts Options) []byte {
	w := columns(cfg.ReceiptWidth)

	var b bytes.Buffer
	Init(&b)
	Header(&b, cfg, opts)
	Title(&b, cfg, "WARRANTY REPLACEMENT", w)

	// --- Body (left) ---
	b.Write([]byte{esc, 'a', 0})
	divider(&b, cfg, w)
	line(&b, leftRight("Date:", datetime.Date(time.Now()), w))
	if s.CustomerName != "" {
		line(&b, leftRight("Customer:", ascii(s.CustomerName), w))
	}
	for _, ln := range wrap("Product: "+ascii(s.ProductName), w) {
		line(&b, ln)
	}
	divider(&b, cfg, w)
	for _, ln := range wrap("Returned serial: "+ascii(s.OldSerial), w) {
		line(&b, ln)
	}
	b.Write([]byte{esc, 'E', 1})
	for _, ln := range wrap("New serial: "+ascii(s.NewSerial), w) {
		line(&b, ln)
	}
	b.Write([]byte{esc, 'E', 0})
	if s.WarrantyUntil != "" {
		until := s.WarrantyUntil
		if s.WarrantyLeft != "" {
			until += " (" + s.WarrantyLeft + ")"
		}
		line(&b, leftRight("Warranty until:", ascii(until), w))
	}
	divider(&b, cfg, w)

	// --- Footer (centered) ---
	b.Write([]byte{esc, 'a', 1})
	line(&b, "Replacement slip - please retain.")
	Footer(&b, cfg)

	return b.Bytes()
}

// DebtSlip is the data for a printed credit payment slip — the proof a
// customer is handed after making a payment toward their outstanding balance.
type DebtSlip struct {
	ReceiptNo, Date, CustomerName, CustomerPhone, Method, CashierName string
	Amount                                                            decimal.Decimal
	BalanceBefore, BalanceAfter                                       *decimal.Decimal
}

// DebtDocument builds the ESC/POS byte stream for a credit payment slip.
// It mirrors the receipt header/footer and lists the payment details and
// updated balance (if available).
func DebtDocument(s DebtSlip, cfg settings.Settings, opts Options) []byte {
	w := columns(cfg.ReceiptWidth)
	sym := cfg.CurrencySymbol
	if sym == "" {
		sym = "Rs."
	}
	var b bytes.Buffer
	Init(&b)
	Header(&b, cfg, opts)
	Title(&b, cfg, "CREDIT PAYMENT", w)
	// meta
	b.Write([]byte{esc, 'a', 0})
	divider(&b, cfg, w)
	line(&b, leftRight("Receipt:", s.ReceiptNo, w))
	line(&b, leftRight("Date:", s.Date, w))
	line(&b, leftRight("Customer:", ascii(s.CustomerName), w))
	if s.CustomerPhone != "" {
		line(&b, leftRight("Phone:", ascii(s.CustomerPhone), w))
	}
	divider(&b, cfg, w)
	// amount
	b.Write([]byte{esc, 'E', 1})
	line(&b, leftRight("Amount paid", money.Format(sym, s.Amount), w))
	b.Write([]byte{esc, 'E', 0})
	line(&b, leftRight("Method:", s.Method, w))
	// balances (omitted for backfilled rows)
	if s.BalanceBefore != nil && s.BalanceAfter != nil {
		divider(&b, cfg, w)
		line(&b, leftRight("Previous balance", money.Format(sym, *s.BalanceBefore), w))
		b.Write([]byte{esc, 'E', 1})
		line(&b, leftRight("Remaining balance", money.Format(sym, *s.BalanceAfter), w))
		b.Write([]byte{esc, 'E', 0})
	}
	divider(&b, cfg, w)
	b.Write([]byte{esc, 'a', 1})
	if s.CashierName != "" {
		line(&b, "Served by: "+ascii(s.CashierName))
	}
	Footer(&b, cfg)
	return b.Bytes()
}

// plural is "1 item" / "3 items".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
