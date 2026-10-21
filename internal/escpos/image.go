package escpos

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
)

// Image printing: the browser draws the slip at the printer's dot width and
// sends it as a 1-bit PNG; ImageJob frames it like a Classic slip (init … feed,
// cut) so only the layout changes, never what the printer is asked to do.

// ErrBadImage is a receipt picture that can't be printed as it is.
var ErrBadImage = errors.New("receipt picture can't be printed")

const (
	maxImageBytes = 8 << 20
	maxImageRows  = 32000 // ~4 m of paper
	bandRows      = 256   // rows per GS v 0: some printers choke on one huge raster
)

// ReadImage decodes a receipt PNG and checks it is exactly dots wide and 1 to
// 32,000 rows tall. Dimensions are checked before the pixels are decoded.
func ReadImage(r io.Reader, dots int) (image.Image, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	if err != nil || len(raw) > maxImageBytes {
		return nil, fmt.Errorf("%w: too big", ErrBadImage)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: not a PNG", ErrBadImage)
	}
	if cfg.Width != dots || cfg.Height < 1 || cfg.Height > maxImageRows {
		return nil, fmt.Errorf("%w: %d×%d on a %d-dot printer", ErrBadImage, cfg.Width, cfg.Height, dots)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadImage, err)
	}
	return img, nil
}

// ImageJob is a whole slip from a picture: init → the picture as GS v 0 bands
// → feed past the cutter → cut. A pixel darker than mid-grey (luminance < 160)
// is ink. No drawer kick: the handler prepends DrawerKick by its own rule.
func ImageJob(img image.Image, dots int) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() != dots || b.Dy() < 1 || b.Dy() > maxImageRows {
		return nil, fmt.Errorf("%w: %d×%d on a %d-dot printer", ErrBadImage, b.Dx(), b.Dy(), dots)
	}
	var out bytes.Buffer
	out.Write([]byte{esc, '@'})
	for y0 := 0; y0 < b.Dy(); y0 += bandRows {
		h := min(bandRows, b.Dy()-y0)
		out.Write(Raster(dots, h, func(x, y int) bool {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y0+y).RGBA()
			if a < 0x8000 {
				return false // transparent = paper
			}
			return (299*r+587*g+114*bl)/1000 < 160*0x101
		}))
	}
	out.Write([]byte{esc, 'd', feedBeforeCut})
	out.Write([]byte{gs, 'V', 1})
	return out.Bytes(), nil
}

// Raster encodes a w×h 1-bit bitmap as an ESC/POS GS v 0 raster command.
// ink(x,y) reports whether a dot should be black. Rows are packed MSB-first,
// ceil(w/8) bytes per row.
func Raster(w, h int, ink func(x, y int) bool) []byte {
	if w <= 0 || h <= 0 {
		return nil
	}
	bytesPerRow := (w + 7) / 8
	out := make([]byte, 0, 8+bytesPerRow*h)
	out = append(out, gs, 'v', '0', 0,
		byte(bytesPerRow%256), byte(bytesPerRow/256),
		byte(h%256), byte(h/256))
	for y := 0; y < h; y++ {
		for bx := 0; bx < bytesPerRow; bx++ {
			var v byte
			for bit := 0; bit < 8; bit++ {
				if x := bx*8 + bit; x < w && ink(x, y) {
					v |= 1 << (7 - bit)
				}
			}
			out = append(out, v)
		}
	}
	return out
}
