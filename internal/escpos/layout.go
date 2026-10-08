package escpos

import (
	"bytes"
	"strings"

	"github.com/shopspring/decimal"
)

// The suite's own receipt layout (not karots-pos's): a ticket strip, an
// ITEM / QTY / AMOUNT table, a right-hand totals block and a scannable receipt
// barcode. These helpers draw its parts; every receipt style keeps its own
// rules, header and total treatment on top.

// itemCols is the table's column widths for a roll of w columns: name, qty,
// amount (two spaces between columns).
func itemCols(w int) (name, qty, amt int) {
	qty, amt = 6, 11
	if w < 40 { // 58 mm
		qty, amt = 4, 9
	}
	return w - qty - amt - 2, qty, amt
}

// qtyText prints a quantity without trailing zeros ("1", "2.25").
func qtyText(q decimal.Decimal) string { return q.String() }

// padLeft right-aligns s in n columns (s longer than n is returned as is).
func padLeft(s string, n int) string {
	if r := len([]rune(s)); r < n {
		return strings.Repeat(" ", n-r) + s
	}
	return s
}

// padRight left-aligns s in n columns.
func padRight(s string, n int) string {
	if r := len([]rune(s)); r < n {
		return s + strings.Repeat(" ", n-r)
	}
	return s
}

// tableHead prints the item table's column titles.
func tableHead(b *bytes.Buffer, w int) {
	nw, qw, aw := itemCols(w)
	b.Write([]byte{esc, 'E', 1})
	line(b, padRight("ITEM", nw)+" "+padLeft("QTY", qw)+" "+padLeft("AMOUNT", aw))
	b.Write([]byte{esc, 'E', 0})
}

// itemRow prints a line's name (wrapped to the name column) with its quantity
// and amount on the first line.
func itemRow(b *bytes.Buffer, w int, name, qty, amount string) {
	nw, qw, aw := itemCols(w)
	lines := wrap(name, nw)
	if len(lines) == 0 {
		lines = []string{""}
	}
	line(b, padRight(lines[0], nw)+" "+padLeft(qty, qw)+" "+padLeft(amount, aw))
	for _, ln := range lines[1:] {
		line(b, ln)
	}
}

// sumRow prints a totals row right-aligned under the amount column, its label
// right-aligned just before it, so the totals read as one block on the right.
func sumRow(b *bytes.Buffer, w int, label, amount string) {
	aw := 16 // room for "Rs. 1,234,567.00"
	if w < 40 {
		aw = 13
	}
	lw := w - aw - 1
	line(b, padLeft(label, lw)+" "+padLeft(amount, aw))
}

// receiptBarcode prints the receipt number as a CODE39 barcode with the number
// under it, so a return or reprint at the counter is one scan. CODE39 takes
// "S-00184" as is (CODE128 needs a code-set prefix some printers print
// literally). ESC/POS: GS h (height), GS w (module width), GS H 2 (text
// below), GS k 69 n data. A number CODE39 can't carry is skipped.
func receiptBarcode(b *bytes.Buffer, code string) {
	code = strings.ToUpper(ascii(code))
	if code == "" || len(code) > 30 || strings.Trim(code, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-. $/+%") != "" {
		return
	}
	b.Write([]byte{esc, 'a', 1}) // center
	b.Write([]byte{gs, 'h', 60})
	b.Write([]byte{gs, 'w', 2})
	b.Write([]byte{gs, 'H', 2})
	b.Write([]byte{gs, 'k', 69, byte(len(code))})
	b.WriteString(code)
	b.Write([]byte{'\n'})
	b.Write([]byte{esc, 'a', 0}) // back to left
}
