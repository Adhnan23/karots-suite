package tspl

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
)

// pattern is a 1-bit test picture: ink where (x*3+y)%5 == 0.
func pattern(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(255)
			if (x*3+y)%5 == 0 {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func TestImageDocumentRoundTrip(t *testing.T) {
	img := pattern(400, 200)
	doc, err := ImageDocument(img, 50, 25, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	for _, want := range []string{"SIZE 50 mm, 25 mm\r\n", "GAP 2 mm, 0 mm\r\n", "CLS\r\n", "PRINT 3,1\r\n"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %q…", want, s[:min(len(s), 200)])
		}
	}
	head := []byte("BITMAP 0,0,50,200,0,")
	i := bytes.Index(doc, head)
	if i < 0 {
		t.Fatal("no BITMAP command")
	}
	data := doc[i+len(head) : i+len(head)+50*200]
	if !bytes.HasPrefix(doc[i+len(head)+50*200:], []byte("\r\nPRINT")) {
		t.Fatal("bitmap is not exactly 50×200 bytes")
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 400; x++ {
			ink := data[y*50+x/8]&(0x80>>(x%8)) == 0 // TSPL: bit 0 prints
			if want := img.GrayAt(x, y).Y == 0; ink != want {
				t.Fatalf("dot %d,%d: ink=%v want %v", x, y, ink, want)
			}
		}
	}
}

func TestImageDocumentRefusesWrongSize(t *testing.T) {
	for _, sz := range [][2]int{{401, 200}, {400, 199}, {576, 200}} {
		if _, err := ImageDocument(pattern(sz[0], sz[1]), 50, 25, 2, 1); !errors.Is(err, ErrBadImage) {
			t.Errorf("%v: %v", sz, err)
		}
	}
}
