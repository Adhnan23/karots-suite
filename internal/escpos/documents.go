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

	// --- Meta (left) ---
	b.Write([]byte{esc, 'a', 0}) // left
	divider(&b, cfg, w)
	line(&b, leftRight("Receipt:", d.Sale.ReceiptNo, w))
	line(&b, leftRight("Date:", datetime.DateTime(d.Sale.CreatedAt), w))
	line(&b, leftRight("Cashier:", ascii(d.Sale.CashierName), w))
	if d.Sale.CustomerName != nil && *d.Sale.CustomerName != "" {
		line(&b, leftRight("Customer:", ascii(*d.Sale.CustomerName), w))
	}
	divider(&b, cfg, w)

	// --- Items ---
	itemDisc := printItems(&b, d, w, opts)
	divider(&b, cfg, w)

	// --- Totals ---
	// Sale.Discount holds item discounts + bill discount; split them so the
	// receipt mirrors the cashier screen (item discounts shown per line above).
	// Payout lines (negative) are money handed over: kept out of the subtotal and
	// shown on their own; a sale they take below zero ends in GIVEN, not TOTAL.
	given := decimal.Zero
	for _, it := range d.Items {
		if it.Subtotal.IsNegative() {
			given = given.Sub(it.Subtotal)
		}
	}
	line(&b, leftRight("Subtotal", money.Format(sym, d.Sale.Subtotal.Add(given)), w))
	if itemDisc.IsPositive() {
		line(&b, leftRight("Item savings", "-"+money.Format(sym, itemDisc), w))
	}
	if billDisc := d.Sale.Discount.Sub(itemDisc); billDisc.IsPositive() {
		line(&b, leftRight("Bill saving"+discSuffix(d.Sale.DiscountType, d.Sale.DiscountValue), "-"+money.Format(sym, billDisc), w))
	}
	if d.Sale.Tax.IsPositive() {
		line(&b, leftRight("Tax", money.Format(sym, d.Sale.Tax), w))
	}
	if given.IsPositive() {
		line(&b, leftRight("Given to customer", "-"+money.Format(sym, given), w))
	}
	// TOTAL — emphasized and set off with blank lines so it stands out.
	line(&b, "")
	if !d.Sale.Total.IsNegative() {
		bigLine(&b, cfg, "TOTAL", money.Format(sym, d.Sale.Total), w)
	}
	if d.Sale.Discount.IsPositive() {
		line(&b, leftRight("You saved", money.Format(sym, d.Sale.Discount), w))
	}

	// Payment breakdown: one line per tender (cash/card/online/...). This
	// replaces the old single "Paid" row — the per-method lines already show it.
	if len(d.Payments) > 0 {
		line(&b, "")
		for _, p := range d.Payments {
			if p.Amount.IsZero() {
				continue // a payout sale's empty cash row
			}
			line(&b, leftRight(capitalize(ascii(p.Method)), money.Format(sym, p.Amount), w))
		}
	}

	// Change/Due last, emphasized. A credit sale shows what is still owed (Due)
	// instead of Change, plus the customer's account after it: TOTAL DUE when they
	// owe, ADVANCE LEFT when a prior advance covered the sale (with the part it
	// paid shown as "From advance").
	if cs, ok := d.Sale.Credit(opts.CustomerDue); ok {
		line(&b, "")
		if cs.FromAdvance.IsPositive() {
			line(&b, leftRight("From advance", "-"+money.Format(sym, cs.FromAdvance), w))
		}
		if due := cs.Due.Sub(cs.FromAdvance); due.IsPositive() {
			bigLine(&b, cfg, "DUE", money.Format(sym, due), w)
		}
		if cs.Balance.IsNegative() {
			bigLine(&b, cfg, "ADVANCE LEFT", money.Format(sym, cs.Balance.Neg()), w)
		} else {
			bigLine(&b, cfg, "TOTAL DUE", money.Format(sym, cs.Balance), w)
		}
	} else if d.Sale.ChangeGiven.IsPositive() {
		line(&b, "")
		label := "CHANGE"
		if d.Sale.Total.IsNegative() {
			label = "GIVEN"
		}
		bigLine(&b, cfg, label, money.Format(sym, d.Sale.ChangeGiven), w)
	}
	// Rounding: cash the customer left behind (paid - change - total). It stayed
	// in the drawer; shown here so the slip explains why cash out < change due —
	// booked to an account customer's balance (On account) or kept as a rounding
	// gain (walk-in money, tied to no one).
	if kept := d.Sale.PaidAmount.Sub(d.Sale.ChangeGiven).Sub(d.Sale.Total); kept.IsPositive() {
		if d.Sale.RoundingToAccount.IsPositive() {
			line(&b, leftRight("On account", money.Format(sym, d.Sale.RoundingToAccount), w))
		}
		if cash := kept.Sub(d.Sale.RoundingToAccount); cash.IsPositive() {
			line(&b, leftRight("Change kept", money.Format(sym, cash), w))
		}
	}
	divider(&b, cfg, w)

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
