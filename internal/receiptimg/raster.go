// Package receiptimg renders the bits of a thermal receipt that the printer's
// built-in font cannot draw — the shop logo and the secondary (non-Latin) shop
// name — into ESC/POS raster blocks (GS v 0). The output is embedded directly
// in the receipt byte stream by internal/escpos.
package receiptimg

// PrinterDots is the print head width in dots for the configured paper. 80mm
// POS printers are 576 dots (72mm @ 203dpi); 58mm are 384.
func PrinterDots(width string) int {
	if width == "58" {
		return 384
	}
	return 576
}
