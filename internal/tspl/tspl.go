// Package tspl renders a barcode price label as a TSPL/TSPL2 byte stream for a
// thermal label printer (e.g. Xprinter XP-365B). Like internal/escpos it uses
// the printer's built-in commands — the BARCODE command draws a scannable
// barcode natively, so no image/barcode library is needed and the binary stays
// dependency-free. The stream is sent raw via internal/printing.Raw.
package tspl

import (
	"fmt"
	"strconv"
	"strings"
)

// dotsPerMM is the resolution of the XP-365B class printers (203 dpi).
const dotsPerMM = 8

// narrow is the barcode's narrow-module width in dots (2 = 0.25mm, scannable).
const narrow = 2

// Input describes a single label run (one design, printed Count times).
type Input struct {
	Top     string // free text above the barcode (e.g. name or a model no); blank = omit
	Bottom  string // free text below the barcode (e.g. price); blank = omit
	NameLines   int    // max wrapped lines for Top (1–4, default 1)
	BottomLines int    // max wrapped lines for Bottom (1–4, default 1)
	NameFont    string // TSPL built-in font for Top: "1"|"2"|"3"|"4" (default "2")
	BottomFont  string // TSPL built-in font for Bottom (default "3")
	Code    string // barcode value
	Format  string // CODE128 | EAN13 | EAN8 | UPC | CODE39 (defaults to CODE128)
	Count   int    // number of copies (>=1)
	WidthMM int    // label width  (mm)
	HeightMM int   // label height (mm)
	GapMM   int    // gap between labels (mm)
}

// barcodeType maps the UI format name to the TSPL BARCODE type token.
func barcodeType(format string) string {
	switch strings.ToUpper(strings.TrimSpace(format)) {
	case "EAN13":
		return "EAN13"
	case "EAN8":
		return "EAN8"
	case "UPC", "UPCA", "UPC-A":
		return "UPCA"
	case "CODE39", "39":
		return "39"
	default:
		return "128" // CODE128
	}
}

// Document builds the TSPL byte stream for the label run.
func Document(in Input) []byte {
	w := in.WidthMM
	if w <= 0 {
		w = 50
	}
	h := in.HeightMM
	if h <= 0 {
		h = 25
	}
	gap := in.GapMM
	if gap < 0 {
		gap = 0
	}
	count := in.Count
	if count < 1 {
		count = 1
	}

	widthDots := w * dotsPerMM
	heightDots := h * dotsPerMM

	// Layout margins (dots). Content is laid out top-down with DIRECTION 0 so the
	// label's leading edge is the top — origin (0,0) is the top-left corner.
	const (
		marginX      = 16
		marginTop    = 12
		marginBottom = 12
		readableH    = 26 // height of the barcode's human-readable digits
	)

	var b strings.Builder
	fmt.Fprintf(&b, "SIZE %d mm, %d mm\r\n", w, h)
	fmt.Fprintf(&b, "GAP %d mm, 0 mm\r\n", gap)
	b.WriteString("DIRECTION 0\r\n")
	b.WriteString("REFERENCE 0,0\r\n")
	b.WriteString("CLS\r\n")

	// Top and bottom are free-text slots — the UI chooses what goes where (name on
	// top & price below, or the reverse). Each is centered and wraps onto up to
	// its configured line count (NameLines / BottomLines, 1–4, default 1) instead
	// of being clipped off the right edge; the barcode between them shrinks to fit
	// (see barH). The font size of each slot is configurable (NameFont/BottomFont,
	// default "2"/"3"); the wrap math tracks the chosen font so centering holds.
	topTok, topCharW, topLineH := fontMetrics(in.NameFont, "2")
	botTok, botCharW, botLineH := fontMetrics(in.BottomFont, "3")
	topLines := wrapLabelText(ascii(in.Top), (widthDots-2*marginX)/topCharW, clampLines(in.NameLines))
	botLines := wrapLabelText(ascii(in.Bottom), (widthDots-2*marginX)/botCharW, clampLines(in.BottomLines))
	nameH := len(topLines) * topLineH
	priceH := len(botLines) * botLineH

	for i, ln := range topLines {
		x := centerX(widthDots, len(ln)*topCharW, marginX)
		fmt.Fprintf(&b, "TEXT %d,%d,\"%s\",0,1,1,\"%s\"\r\n", x, marginTop+i*topLineH, topTok, tsplEscape(ln))
	}

	// Barcode (centered) fills the space between the top text and the reserved
	// bottom area, leaving room for its own human-readable digits underneath.
	code := tsplEscape(ascii(in.Code))
	barY := marginTop + nameH
	barH := heightDots - barY - readableH - priceH - marginBottom
	if barH < 40 {
		barH = 40
	}
	barX := centerX(widthDots, barcodeWidthDots(in.Format, ascii(in.Code), narrow), marginX)
	fmt.Fprintf(&b, "BARCODE %d,%d,\"%s\",%d,1,0,%d,%d,\"%s\"\r\n",
		barX, barY, barcodeType(in.Format), barH, narrow, narrow*2, code)

	for i, ln := range botLines {
		x := centerX(widthDots, len(ln)*botCharW, marginX)
		priceY := barY + barH + readableH + i*botLineH
		fmt.Fprintf(&b, "TEXT %d,%d,\"%s\",0,1,1,\"%s\"\r\n", x, priceY, botTok, tsplEscape(ln))
	}

	fmt.Fprintf(&b, "PRINT %s,1\r\n", strconv.Itoa(count))
	return []byte(b.String())
}

// fontMetrics returns the TSPL built-in font token plus its per-character width
// and reserved per-line height (dots, at 1× scale) for a size key. Blank/unknown
// falls back to def. Smaller fonts free up room for more lines / a taller
// barcode; larger fonts are more readable but leave less space.
//
//	"1" 8×12  "2" 12×20  "3" 16×24  "4" 24×32   (line height = glyph height + spacing)
func fontMetrics(key, def string) (token string, charW, lineH int) {
	if key == "" {
		key = def
	}
	switch key {
	case "1":
		return "1", 8, 18
	case "3":
		return "3", 16, 30
	case "4":
		return "4", 24, 40
	default: // "2"
		return "2", 12, 26
	}
}

// clampLines bounds a configured line count to 1–4, defaulting a zero/negative
// value to 1 (a single line, the historical behaviour).
func clampLines(n int) int {
	if n < 1 {
		return 1
	}
	if n > 4 {
		return 4
	}
	return n
}

// centerX returns the x offset that horizontally centers content of the given
// width on a label widthDots wide, never less than the left margin.
func centerX(widthDots, contentWidth, margin int) int {
	x := (widthDots - contentWidth) / 2
	if x < margin {
		x = margin
	}
	return x
}

// barcodeWidthDots estimates the printed width of a barcode so it can be
// centered. Widths come from each symbology's encoding; CODE128 accounts for the
// digit-pair (Code C) compression the printer applies to all-numeric data.
func barcodeWidthDots(format, code string, narrowDots int) int {
	switch barcodeType(format) {
	case "EAN13", "UPCA":
		return 95 * narrowDots // 95 modules
	case "EAN8":
		return 67 * narrowDots
	case "39":
		// Code39: ~13 narrow-equivalents per char incl. wide bars and the gap,
		// plus the start/stop (*) characters.
		return (len(code) + 2) * 13 * narrowDots
	default: // CODE128
		symbols := len(code)
		if isAllDigits(code) {
			symbols = (len(code) + 1) / 2 // Code C packs two digits per symbol
		}
		modules := 11*(symbols+2) + 13 // start + data + checksum (11 each) + stop (13)
		return modules * narrowDots
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// wrapLabelText greedily word-wraps s into at most maxLines lines of at most
// maxChars characters each (character count = font "2" columns). Words longer
// than a line are hard-split; anything past maxLines is dropped, so a line never
// overflows the label width. Returns nil for blank input.
func wrapLabelText(s string, maxChars, maxLines int) []string {
	if maxChars < 1 {
		maxChars = 1
	}
	// Break into tokens that each fit on a line (hard-split over-long words).
	var tokens []string
	for _, w := range strings.Fields(s) {
		for len(w) > maxChars {
			tokens = append(tokens, w[:maxChars])
			w = w[maxChars:]
		}
		if w != "" {
			tokens = append(tokens, w)
		}
	}
	// Greedily pack tokens into lines.
	var lines []string
	cur := ""
	for _, t := range tokens {
		switch {
		case cur == "":
			cur = t
		case len(cur)+1+len(t) <= maxChars:
			cur += " " + t
		default:
			lines = append(lines, cur)
			cur = t
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

// tsplEscape escapes the characters that are special inside a TSPL quoted
// string: a backslash and a double quote are written as \\ and \".
func tsplEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// ascii keeps printable ASCII and replaces anything else with '?', so the
// printer's built-in font never renders garbage for non-Latin text. (Use the
// HTML label sheet if you need non-Latin glyphs.)
func ascii(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x20 && r < 0x7F:
			b.WriteRune(r)
		case r == '\n' || r == '\t':
			b.WriteByte(' ')
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}
