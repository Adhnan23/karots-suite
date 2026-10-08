package escpos

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
)

// PreviewLine is one printed line of a slip as the paper shows it: the text
// (spacing kept, so padded columns and centring stay true) with the print mode
// it was printed in, or a raster image (logo, non-Latin shop name).
type PreviewLine struct {
	Text      string
	Align     string // "left" | "center" | "right"
	Bold      bool
	Wide      bool // double width
	Tall      bool // double height
	Small     bool // Font B (the credit line)
	Image     string
	ImageDots int // raster width in printer dots
}

// Preview decodes the exact ESC/POS bytes a slip sends to the printer back into
// lines, so an on-screen preview can never differ from the paper. It knows the
// commands this package and receiptimg emit; the drawer kick, feeds and anything
// after the cut don't show.
func Preview(raw []byte) []PreviewLine {
	var out []PreviewLine
	st := PreviewLine{Align: "left"}
	var cur []byte
	flush := func() {
		l := st
		l.Text = string(cur) // spaces kept: the printer centres the whole padded line
		out = append(out, l)
		cur = cur[:0]
	}
	arg := func(i, k int) byte { // the k-th parameter byte after a 2-byte command at i
		if i+1+k < len(raw) {
			return raw[i+1+k]
		}
		return 0
	}
	for i := 0; i < len(raw); i++ {
		switch c := raw[i]; c {
		case esc:
			if i+1 >= len(raw) {
				return out
			}
			switch raw[i+1] {
			case '@':
				st = PreviewLine{Align: "left"}
				i++
			case 'a':
				st.Align = [...]string{"left", "center", "right"}[arg(i, 1)%3]
				i += 2
			case 'E':
				st.Bold = arg(i, 1)&1 == 1
				i += 2
			case 'M':
				st.Small = arg(i, 1)&1 == 1
				i += 2
			case 'p': // drawer kick: m t1 t2
				i += 4
			default: // t, d, -, J …: one parameter
				i += 2
			}
		case gs:
			if i+1 >= len(raw) {
				return out
			}
			switch raw[i+1] {
			case '!':
				n := arg(i, 1)
				st.Wide, st.Tall = n&0xF0 != 0, n&0x0F != 0
				i += 2
			case 'B': // reverse print: shown as bold
				st.Bold = st.Bold || arg(i, 1)&1 == 1
				i += 2
			case 'V': // cut: the slip ends here
				if len(cur) > 0 {
					flush()
				}
				return out
			case 'v': // GS v 0 m xL xH yL yH data
				if i+7 >= len(raw) {
					return out
				}
				xb := int(raw[i+4]) | int(raw[i+5])<<8
				h := int(raw[i+6]) | int(raw[i+7])<<8
				end := i + 8 + xb*h
				if end > len(raw) {
					return out
				}
				if len(cur) > 0 {
					flush()
				}
				out = append(out, PreviewLine{Align: st.Align, Image: rasterPNG(raw[i+8:end], xb, h), ImageDots: xb * 8})
				i = end - 1
			default:
				i += 2
			}
		case '\n':
			flush()
		case '\r':
		default:
			cur = append(cur, c)
		}
	}
	if len(cur) > 0 {
		flush()
	}
	return out
}

// rasterPNG turns a GS v 0 bitmap (rows MSB-first, 1 = ink) into a PNG data URI.
func rasterPNG(data []byte, xb, h int) string {
	img := image.NewGray(image.Rect(0, 0, xb*8, h))
	for y := 0; y < h; y++ {
		for x := 0; x < xb*8; x++ {
			v := uint8(255)
			if data[y*xb+x/8]&(0x80>>(x%8)) != 0 {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
