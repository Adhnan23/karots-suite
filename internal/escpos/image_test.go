package escpos

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// pattern is a 1-bit test picture: a diagonal plus text-like blocks, so a lost,
// shifted or flipped row or column shows.
func pattern(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ink := x == y%w || (y/24%3 == 0 && x/12%5 < 3 && y%24 < 20)
			v := uint8(255)
			if ink {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func TestImageJobRoundTrip(t *testing.T) {
	in := pattern(576, 2100)
	job, err := ImageJob(in, 576)
	if err != nil {
		t.Fatal(err)
	}
	lines := Preview(job)
	var bands []image.Image
	for _, l := range lines {
		if l.Image == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(l.Image, "data:image/png;base64,"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if b.Bounds().Dy() > 256 {
			t.Fatalf("band of %d rows", b.Bounds().Dy())
		}
		bands = append(bands, b)
	}
	if len(bands) < 9 {
		t.Fatalf("%d bands", len(bands))
	}
	y0 := 0
	for _, b := range bands {
		for y := 0; y < b.Bounds().Dy(); y++ {
			for x := 0; x < 576; x++ {
				r, _, _, _ := b.At(x, y).RGBA()
				if got, want := r < 0x8000, in.GrayAt(x, y0+y).Y < 128; got != want {
					t.Fatalf("pixel %d,%d: ink %v, want %v", x, y0+y, got, want)
				}
			}
		}
		y0 += b.Bounds().Dy()
	}
	if y0 != 2100 {
		t.Fatalf("%d rows back, want 2100", y0)
	}
}

func TestImageJobFrame(t *testing.T) {
	job, err := ImageJob(pattern(384, 40), 384)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(job, []byte{esc, '@'}) {
		t.Fatalf("starts % x", job[:4])
	}
	if !bytes.HasSuffix(job, []byte{esc, 'd', feedBeforeCut, gs, 'V', 1}) {
		t.Fatalf("ends % x", job[len(job)-6:])
	}
	if bytes.Contains(job, []byte{esc, 'p'}) {
		t.Fatal("an image job never carries the drawer kick; the handler adds it")
	}
}

func TestReadImage(t *testing.T) {
	enc := func(w, h int) []byte {
		var b bytes.Buffer
		png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))) //nolint:errcheck
		return b.Bytes()
	}
	for name, c := range map[string]struct {
		body []byte
		dots int
	}{
		"too narrow": {enc(575, 10), 576},
		"too wide":   {enc(576, 10), 384},
		"too tall":   {enc(384, 32001), 384},
		"not a png":  {[]byte("hello"), 576},
	} {
		if _, err := ReadImage(bytes.NewReader(c.body), c.dots); !errors.Is(err, ErrBadImage) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := ReadImage(bytes.NewReader(enc(384, 32000)), 384); err != nil {
		t.Fatalf("384 on 58 mm: %v", err)
	}
}
