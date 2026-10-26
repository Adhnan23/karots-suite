package tspl

import (
	"errors"
	"fmt"
	"image"
	"strconv"
	"strings"
)

// ErrBadImage is a label picture that isn't exactly the label's size in dots.
var ErrBadImage = errors.New("label picture can't be printed")

// ImageDocument prints a label drawn by the browser (Image mode): the picture
// is the whole label, exactly widthMM×heightMM at 8 dots per mm, sent as one
// TSPL BITMAP and printed count times. A dot darker than mid-grey (luminance
// < 160, the slip rule) is ink; a see-through dot is paper.
func ImageDocument(img image.Image, widthMM, heightMM, gapMM, count int) ([]byte, error) {
	w, h := widthMM*dotsPerMM, heightMM*dotsPerMM
	b := img.Bounds()
	if w <= 0 || h <= 0 || b.Dx() != w || b.Dy() != h {
		return nil, fmt.Errorf("%w: %d×%d on a %d×%d-dot label", ErrBadImage, b.Dx(), b.Dy(), w, h)
	}
	rowBytes := (w + 7) / 8
	bits := make([]byte, rowBytes*h)
	for i := range bits {
		bits[i] = 0xff // TSPL: 1 = paper, 0 = ink
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if ink(img, b.Min.X+x, b.Min.Y+y) {
				bits[y*rowBytes+x/8] &^= 0x80 >> (x % 8)
			}
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "SIZE %d mm, %d mm\r\n", widthMM, heightMM)
	fmt.Fprintf(&out, "GAP %d mm, 0 mm\r\n", max(gapMM, 0))
	out.WriteString("DIRECTION 0\r\nREFERENCE 0,0\r\nCLS\r\n")
	fmt.Fprintf(&out, "BITMAP 0,0,%d,%d,0,", rowBytes, h)
	out.Write(bits)
	fmt.Fprintf(&out, "\r\nPRINT %s,1\r\n", strconv.Itoa(max(count, 1)))
	return []byte(out.String()), nil
}

func ink(img image.Image, x, y int) bool {
	r, g, bl, a := img.At(x, y).RGBA()
	if a < 0x8000 {
		return false
	}
	return 299*(r>>8)+587*(g>>8)+114*(bl>>8) < 160*1000
}
