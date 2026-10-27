package printer

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"karots-suite/internal/escpos"
	"karots-suite/internal/tspl"
)

// pattern is a w×h picture with a diagonal and a solid top-left block of ink.
func pattern(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.Gray{Y: 255}
			if x == y%w || (x < 20 && y < 10) {
				c = color.Gray{Y: 0}
			}
			img.SetGray(x, y, c)
		}
	}
	return img
}

func samePixels(t *testing.T, want *image.Gray, gotPNG []byte) {
	t.Helper()
	got, err := png.Decode(bytes.NewReader(gotPNG))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if got.Bounds().Dx() != want.Bounds().Dx() || got.Bounds().Dy() != want.Bounds().Dy() {
		t.Fatalf("size %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			r, _, _, _ := got.At(x, y).RGBA()
			if (r < 0x8000) != (want.GrayAt(x, y).Y < 128) {
				t.Fatalf("pixel %d,%d differs", x, y)
			}
		}
	}
}

func TestDecodeESCPOSRoundTrip(t *testing.T) {
	img := pattern(576, 300) // more than one GS v 0 band
	job, err := escpos.ImageJob(img, 576)
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodeESCPOS(job)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kick {
		t.Fatal("kick without ESC p")
	}
	samePixels(t, img, p.PNG)
}

func TestDecodeESCPOSKick(t *testing.T) {
	img := pattern(384, 20)
	job, _ := escpos.ImageJob(img, 384)
	job = append([]byte{0x1B, 0x70, 0, 0x19, 0xFA}, job...)
	p, err := DecodeESCPOS(job)
	if err != nil || !p.Kick {
		t.Fatalf("kick = %v, err %v", p.Kick, err)
	}
}

func TestDecodeESCPOSClassicText(t *testing.T) {
	job := append([]byte{0x1B, '@'}, []byte("MY SHOP\nTotal 100.00\n")...)
	p, err := DecodeESCPOS(job)
	if err != nil {
		t.Fatal(err)
	}
	if p.PNG != nil || len(p.Text) == 0 {
		t.Fatalf("classic job: png %d bytes, text %q", len(p.PNG), p.Text)
	}
}

func TestDecodeTSPLRoundTrip(t *testing.T) {
	img := pattern(50*8, 30*8)
	job, err := tspl.ImageDocument(img, 50, 30, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodeTSPL(job)
	if err != nil {
		t.Fatal(err)
	}
	if p.Copies != 3 {
		t.Fatalf("copies = %d, want 3", p.Copies)
	}
	samePixels(t, img, p.PNG)
}

func TestDecodeTSPLClassicText(t *testing.T) {
	job := []byte("SIZE 50 mm, 30 mm\r\nCLS\r\nTEXT 10,10,\"3\",0,1,1,\"Rice 5kg\"\r\nPRINT 2,1\r\n")
	p, err := DecodeTSPL(job)
	if err != nil {
		t.Fatal(err)
	}
	if p.PNG != nil || len(p.Text) != 1 || p.Text[0] != "Rice 5kg" || p.Copies != 2 {
		t.Fatalf("classic label: %+v", p)
	}
}
