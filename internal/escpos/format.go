// Package escpos renders a sale as an ESC/POS byte stream for a thermal receipt
// printer (e.g. Xprinter POS-80). It uses the printer's built-in font rather
// than raster graphics, so printing is fast and immune to the PDF/codepage
// garbage that browser printing to a raw CUPS queue produces. The width is
// driven by the shop's receipt_width setting (80mm = 48 cols, 58mm = 32 cols).
package escpos

import (
	"bytes"
	"strings"

	"karots-suite/internal/features/settings"

	"github.com/shopspring/decimal"
)

// columns is the character width of a print line for the configured roll.
func columns(width string) int {
	if width == "58" {
		return 32
	}
	return 48
}

// wrap breaks s into lines of at most w characters, preferring to break at
// spaces. A single word longer than w is hard-split so it never overflows.
func wrap(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		for len(word) > w { // word itself too long: hard-split
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, word[:w])
			word = word[w:]
		}
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= w:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func line(b *bytes.Buffer, s string) {
	b.WriteString(s)
	b.WriteByte('\n')
}

// divider draws the body separator between sections, per the receipt style —
// the receipt's visual skeleton and its biggest differentiator (a solid dash
// rule, a blank gap, a dotted line, or a short rule). Takes cfg so every core
// and plugin slip that calls the exported Divider gets the shop's chosen look.
func divider(b *bytes.Buffer, cfg settings.Settings, w int) {
	switch StyleFor(cfg).Body {
	case "blank": // modern: whitespace instead of a rule
		line(b, "")
	case "dots": // boxed: a dotted separator
		line(b, strings.Repeat(".", w))
	case "compact": // compact chit: a short centered rule
		line(b, center(strings.Repeat("-", w/2), w))
	default: // dash (classic)
		line(b, strings.Repeat("-", w))
	}
}

// bigLine prints a label/amount line emphasised per the style's TotalMode:
// "double" (double-height bold, the character count stays the same so
// leftRight's padding still lines up), "reverse" (a white-on-black bar), or
// "bold" (a plain bold single line). Used for TOTAL, CHANGE, DUE, TOTAL DUE.
func bigLine(b *bytes.Buffer, cfg settings.Settings, l, r string, w int) {
	switch StyleFor(cfg).TotalMode {
	case "reverse": // white-on-black bar (GS B 1 … GS B 0)
		b.Write([]byte{esc, 'E', 1})
		b.Write([]byte{gs, 'B', 1})
		line(b, leftRight(l, r, w))
		b.Write([]byte{gs, 'B', 0})
		b.Write([]byte{esc, 'E', 0})
	case "bold": // plain bold single line
		b.Write([]byte{esc, 'E', 1})
		line(b, leftRight(l, r, w))
		b.Write([]byte{esc, 'E', 0})
	default: // "double": double-height bold
		b.Write([]byte{esc, 'E', 1})   // bold on
		b.Write([]byte{gs, '!', 0x01}) // double height
		line(b, leftRight(l, r, w))
		b.Write([]byte{gs, '!', 0x00}) // normal size
		b.Write([]byte{esc, 'E', 0})   // bold off
	}
}

// center pads s with leading spaces so its text sits centered within a w-column
// line. Centering is baked into the characters (rather than the ESC a 1 hardware
// justify) so it looks identical on a real printer, on the emulator's fixed-width
// canvas, and in plain text — and it centers over the SAME w columns the body
// uses, so the header lines up with the dividers below. A string at or past w is
// returned unchanged.
func center(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", (w-len(s))/2) + s
}

// centerBox centers s within EXACTLY w columns, padding both sides (unlike
// center, which only left-pads). Used for framed-header lines so the right
// border sits flush at the edge. Overlong strings are clipped to w.
func centerBox(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	total := w - len(s)
	left := total / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", total-left)
}

// leftRight pads a left and right label out to w columns. The left side is
// truncated if the two together would overflow the line.
func leftRight(l, r string, w int) string {
	if len(l)+len(r) >= w {
		max := w - len(r) - 1
		if max < 0 {
			max = 0
		}
		if len(l) > max {
			l = l[:max]
		}
	}
	pad := w - len(l) - len(r)
	if pad < 1 {
		pad = 1
	}
	return l + strings.Repeat(" ", pad) + r
}

// pctLabel renders a percentage value without trailing zeros (10.00 -> "10%").
func pctLabel(v decimal.Decimal) string {
	s := v.String()
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s + "%"
}

// discSuffix is the parenthetical shown next to a discount entered as a
// percentage, e.g. " (10%)"; blank for a fixed amount.
func discSuffix(dtype string, value decimal.Decimal) string {
	if dtype == "percent" && value.IsPositive() {
		return " (" + pctLabel(value) + ")"
	}
	return ""
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ascii keeps printable ASCII and replaces anything else (including UTF-8
// multibyte runes) with '?', so the printer's PC437 code page never renders
// the CJK garbage seen when raw bytes are misinterpreted. Non-Latin shop text
// (e.g. Sinhala) won't print on the built-in font — use the HTML receipt for that.
// ASCII sanitises an arbitrary string to the printable ASCII subset the built-in
// thermal font can render (anything else becomes '?'). Exported for callers that
// hand-build slips outside this package (e.g. the cashflow money receipt), so a
// stray em-dash or a non-Latin name can't reach the printer as codepage garbage.
func ASCII(s string) string { return ascii(s) }

func ascii(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r < 0x7F:
			b.WriteRune(r)
		case r == '\n' || r == '\t':
			b.WriteByte(' ')
		// Map the common typographic punctuation to its ASCII equivalent so a
		// stray "smart" character prints as itself, not as '?'. Em/en/figure
		// dashes and the horizontal bar all become a plain hyphen.
		case r == '‐' || r == '‑' || r == '‒' || r == '–' ||
			r == '—' || r == '―' || r == '−':
			b.WriteByte('-')
		case r == '‘' || r == '’' || r == '‚' || r == '′':
			b.WriteByte('\'')
		case r == '“' || r == '”' || r == '„' || r == '″':
			b.WriteByte('"')
		case r == '…': // ellipsis
			b.WriteString("...")
		case r == ' ': // non-breaking space
			b.WriteByte(' ')
		case r == '•' || r == '·': // bullet / middle dot
			b.WriteByte('*')
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
