// Package escpos renders a sale as an ESC/POS byte stream for a thermal receipt
// printer (e.g. Xprinter POS-80). It uses the printer's built-in font rather
// than raster graphics, so printing is fast and immune to the PDF/codepage
// garbage that browser printing to a raw CUPS queue produces. The width is
// driven by the shop's receipt_width setting (80mm = 48 cols, 58mm = 32 cols).
package escpos

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"karots-suite/internal/features/settings"
	"karots-suite/internal/printing"

	"github.com/shopspring/decimal"
)

// DrawerKick returns the ESC/POS pulse that opens the cash drawer wired to the
// receipt printer (ESC p m t1 t2), or nil when the shop hasn't enabled it. m is
// the drawer pin (0 = pin 2, 1 = pin 5); t1/t2 are on/off times in 2 ms units.
func DrawerKick(cfg settings.Settings) []byte {
	if !cfg.OpenCashDrawer {
		return nil
	}
	m := byte(0)
	if cfg.DrawerKickPin == 1 {
		m = 1
	}
	return []byte{0x1B, 0x70, m, 0x19, 0xFA}
}

// KickDrawer sends the drawer pulse to the receipt printer, best-effort. A
// printer that is offline, missing, or has no drawer attached is a silent no-op —
// the caller has already committed the money transaction. No-op when disabled.
func KickDrawer(ctx context.Context, cfg settings.Settings) {
	if b := DrawerKick(cfg); b != nil {
		_ = printing.Raw(ctx, cfg.ReceiptPrinter, b)
	}
}

// ESC/POS control bytes.
const (
	esc = 0x1B
	gs  = 0x1D
)

// feedBeforeCut is how many blank lines to feed before the cut. The cutter sits
// below the print head, so this must cover that gap plus a small bottom margin,
// otherwise the last line is cut off.
const feedBeforeCut = 8

// Options carries pre-rendered raster blocks (ESC/POS GS v 0) that the built-in
// font can't produce: the shop logo and the secondary (non-Latin) shop name.
// Both are optional; nil means "skip". They are produced by internal/receiptimg.
type Options struct {
	Logo    []byte // raster for the logo (top of the receipt)
	SubName []byte // raster for the secondary shop name (under the main name)
	// Serials maps a product id to pre-formatted serial lines for serial-tracked
	// items (e.g. "SN: ABC123 (wty 2027-06-13)"); printed under each matching line.
	Serials map[int64][]string
	// CustomerDue is the customer's outstanding balance after this sale (prior
	// debt + this sale's unpaid portion). Used only on credit/partial sales to
	// print the "Total due" line; the zero value means no balance line.
	CustomerDue decimal.Decimal
}

// Init writes the ESC/POS reset + Latin code page that every receipt starts with.
func Init(b *bytes.Buffer) {
	b.Write([]byte{esc, '@'})    // initialize
	b.Write([]byte{esc, 't', 0}) // code page PC437 (Latin)
}

// Header writes the shared, centered branding block at the top of EVERY receipt —
// logo, the shop name in double-width bold, the secondary-language (Sinhala/Tamil)
// name image, address, phone and VAT no — so a sale, a money receipt and a
// plugin slip all carry identical branding. Text is centered by space-padding
// (see center()); the printer is left in left-align mode for the body.
func Header(b *bytes.Buffer, cfg settings.Settings, opts Options) {
	w := columns(cfg.ReceiptWidth)
	style := StyleFor(cfg)
	// A raster (logo, secondary-language name) can only be centered by the hardware
	// justify command. Text is centered by space-padding instead — see center().
	if len(opts.Logo) > 0 {
		b.Write([]byte{esc, 'a', 1}) // center the raster
		b.Write(opts.Logo)
		b.Write([]byte{esc, 'a', 0}) // back to left for the padded text
		line(b, "")
	}
	name := ascii(cfg.ShopName)
	// Framed look: an ASCII +==+ / | … | box around the whole branding block. Its
	// column maths only hold at normal glyph size, so a framed header never uses
	// the double-size name — the frame itself is the emphasis.
	if style.Framed {
		framedHeader(b, w, name, cfg, opts)
		return
	}
	// Shop name: a double-WIDTH glyph is 2 columns, so its budget is only w/2
	// (16 on a 58mm roll) — center it over w/2. A name wider than that runs off the
	// edge, so drop to double-height-only (full-width glyphs, still bold and tall)
	// and wrap, keeping a long name readable on the narrow roll instead of clipping.
	b.Write([]byte{esc, 'E', 1}) // bold on
	if style.NameDouble && len(name) <= w/2 {
		b.Write([]byte{gs, '!', 0x11}) // double width + height
		line(b, hpad(name, w, style, w/2))
	} else {
		b.Write([]byte{gs, '!', 0x01}) // double height only (also the plain-name styles)
		for _, ln := range wrap(name, w) {
			line(b, hpad(ln, w, style, w))
		}
	}
	b.Write([]byte{gs, '!', 0x00}) // normal size
	b.Write([]byte{esc, 'E', 0})   // bold off
	// Secondary shop name (Sinhala/Tamil) is a raster image (the built-in font
	// can't draw it); printed here at the same size the sale receipt uses.
	if len(opts.SubName) > 0 {
		b.Write([]byte{esc, 'a', 1}) // center the raster
		b.Write(opts.SubName)
		b.Write([]byte{esc, 'a', 0}) // back to left
	}
	if style.Breathing {
		line(b, "") // breathing room between the name and the address block
	}
	if s := deref(cfg.Address); s != "" {
		for _, ln := range wrap(ascii(s), w) {
			line(b, hpad(ln, w, style, w))
		}
	}
	if s := deref(cfg.Phone); s != "" {
		line(b, hpad("Tel: "+ascii(s), w, style, w))
	}
	if cfg.TaxRegistered {
		if s := deref(cfg.TaxRegNo); s != "" {
			line(b, hpad("VAT: "+ascii(s), w, style, w))
		}
	}
	// A full-width rule under the header is part of the receipt style's identity.
	if style.HeaderRule != "" {
		line(b, strings.Repeat(style.HeaderRule, w))
	}
}

// hpad places a header string per the style: left-flush (modern) or centered
// (everything else) over the given budget. Keeps every header line consistent.
func hpad(s string, w int, style ReceiptStyle, budget int) string {
	if style.HeaderLeft {
		return s
	}
	return center(s, budget)
}

// framedHeader draws the branding block inside an ASCII box, normal glyph size
// so the borders line up on the roll. Name in bold, address/phone/VAT stacked
// inside the same frame.
func framedHeader(b *bytes.Buffer, w int, name string, cfg settings.Settings, opts Options) {
	inner := w - 2 // room between the "|" side borders
	border := func() { line(b, "+"+strings.Repeat("=", inner)+"+") }
	// center() only left-pads; a box needs the content padded to the FULL inner
	// width so the closing "|" lands on the right edge, not against the text.
	boxed := func(s string) { line(b, "|"+centerBox(s, inner)+"|") }
	border()
	b.Write([]byte{esc, 'E', 1}) // bold name
	for _, ln := range wrap(name, inner) {
		boxed(ln)
	}
	b.Write([]byte{esc, 'E', 0})
	if s := deref(cfg.Address); s != "" {
		for _, ln := range wrap(ascii(s), inner) {
			boxed(ln)
		}
	}
	if s := deref(cfg.Phone); s != "" {
		boxed("Tel: " + ascii(s))
	}
	if cfg.TaxRegistered {
		if s := deref(cfg.TaxRegNo); s != "" {
			boxed("VAT: " + ascii(s))
		}
	}
	border()
	// Secondary-language raster can't live inside the ASCII box; center it under.
	if len(opts.SubName) > 0 {
		b.Write([]byte{esc, 'a', 1})
		b.Write(opts.SubName)
		b.Write([]byte{esc, 'a', 0})
	}
}

// Title writes a bold, decorated receipt title (e.g. "*** REFUND ***" or
// "[ REFUND ]" depending on the style) under the header, centered by
// space-padding over w columns. An empty title prints nothing — a sale needs no
// title line. cfg selects the decoration (ReceiptStyle.TitleDeco).
func Title(b *bytes.Buffer, cfg settings.Settings, title string, w int) {
	if strings.TrimSpace(title) == "" {
		return
	}
	deco := StyleFor(cfg).TitleDeco
	if !strings.Contains(deco, "%s") {
		deco = "%s"
	}
	line(b, "")
	b.Write([]byte{esc, 'E', 1})
	line(b, center(fmt.Sprintf(deco, strings.ToUpper(ascii(title))), w))
	b.Write([]byte{esc, 'E', 0})
}

// Footer writes the shared, centered footer at the bottom of EVERY receipt — the
// shop's own footer message from Settings, a thank-you line, and the builder
// credit line in the small font — then feeds past the cutter gap and cuts. This
// is what makes every receipt end the same way.
func Footer(b *bytes.Buffer, cfg settings.Settings) {
	w := columns(cfg.ReceiptWidth)
	style := StyleFor(cfg)
	// Reset to left-align first: the footer centers by space-padding (see
	// center()), which only lines up when the printer is NOT already in ESC
	// center mode — otherwise the padded text is centered a second time and drifts
	// right. Callers that ended in Center (e.g. a signature strip on money/debt
	// slips) would otherwise render this off-centre; the sale/repair slips were
	// already left when they got here. Resetting here fixes every caller at once.
	b.Write([]byte{esc, 'a', 0})
	// A matching rule above the footer, per the receipt style's identity.
	if style.HeaderRule != "" {
		line(b, strings.Repeat(style.HeaderRule, w))
	}
	// Centered by space-padding (see center()) so it lines up over the body and
	// renders centered on any printer, the emulator, and plain text alike.
	if s := deref(cfg.ReceiptFooter); s != "" {
		for _, ln := range wrap(ascii(s), w) {
			line(b, center(ln, w))
		}
	}
	if style.ThankYou {
		line(b, center("Thank you! Come again.", w))
	}
	if style.Breathing {
		line(b, "")
	}
	b.Write([]byte{esc, 'M', 1}) // select Font B (smaller)
	// Font B is narrower (~42 cols on a 58mm roll), so pad over the wider budget.
	fw := w * 4 / 3
	line(b, center("Karots Business Suite", fw))
	line(b, center("adhnanmsa@gmail.com | 0769626396", fw))
	b.Write([]byte{esc, 'M', 0}) // back to Font A
	// Feed past the head-to-cutter gap (plus a little margin) before cutting.
	b.Write([]byte{esc, 'd', feedBeforeCut})
	b.Write([]byte{gs, 'V', 1})
}

// Exported thin wrappers so slip builders in other packages (money receipts,
// a plugin) render rows with exactly the same alignment as the sale receipt.
func Line(b *bytes.Buffer, s string)                        { line(b, s) }
func Divider(b *bytes.Buffer, cfg settings.Settings, w int) { divider(b, cfg, w) }
func LeftRight(l, r string, w int) string                   { return leftRight(l, r, w) }
func BigLine(b *bytes.Buffer, cfg settings.Settings, l, r string, w int) {
	bigLine(b, cfg, l, r, w)
}
func Columns(width string) int      { return columns(width) }
func Wrap(s string, w int) []string { return wrap(s, w) }
func Left(b *bytes.Buffer)          { b.Write([]byte{esc, 'a', 0}) }
func Center(b *bytes.Buffer)        { b.Write([]byte{esc, 'a', 1}) }

// Emphasis toggles bold, for a builder that wants to stress one row (e.g. the
// amount) exactly like the sale's TOTAL.
func Emphasis(b *bytes.Buffer, on bool) {
	if on {
		b.Write([]byte{esc, 'E', 1})
	} else {
		b.Write([]byte{esc, 'E', 0})
	}
}
