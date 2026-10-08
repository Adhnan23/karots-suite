package escpos

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreviewDecodesStyleAndRaster(t *testing.T) {
	var b bytes.Buffer
	Init(&b)
	b.Write([]byte{esc, 'p', 0, 25, 250}) // drawer kick: never shows
	Center(&b)
	b.Write([]byte{esc, 'E', 1, gs, '!', 0x11})
	b.WriteString("SHOP\n")
	b.Write([]byte{gs, '!', 0x01})
	b.WriteString("TOTAL  800.00\n")
	b.Write([]byte{gs, '!', 0x00, esc, 'E', 0})
	Left(&b)
	b.WriteString("Widget      500.00  \r\n")
	b.Write([]byte{esc, 'M', 1})
	b.WriteString("Karots Business Suite\n")
	b.Write([]byte{esc, 'M', 0})
	// a 16x2 raster: first row all ink, second row blank
	b.Write([]byte{gs, 'v', '0', 0, 2, 0, 2, 0, 0xFF, 0xFF, 0x00, 0x00})
	b.Write([]byte{esc, 'd', 8, gs, 'V', 1})
	b.WriteString("after the cut\n")

	got := Preview(b.Bytes())
	if len(got) != 5 {
		t.Fatalf("want 5 lines, got %d: %+v", len(got), got)
	}
	if l := got[0]; l.Text != "SHOP" || !l.Bold || !l.Wide || !l.Tall || l.Align != "center" {
		t.Errorf("name line: %+v", l)
	}
	if l := got[1]; l.Wide || !l.Tall || !l.Bold {
		t.Errorf("total line: %+v", l)
	}
	if l := got[2]; l.Text != "Widget      500.00  " || l.Bold || l.Align != "left" {
		t.Errorf("item line: %+v", l)
	}
	if l := got[3]; !l.Small || l.Text != "Karots Business Suite" {
		t.Errorf("credit line: %+v", l)
	}
	if l := got[4]; l.ImageDots != 16 || !strings.HasPrefix(l.Image, "data:image/png;base64,") {
		t.Errorf("raster line: %+v", l)
	}
}

// Every receipt style decodes to clean text: a control byte left in a line means
// a printer command the preview doesn't know, and the screen would show garbage.
func TestPreviewOfRealReceiptsIsClean(t *testing.T) {
	for _, style := range []string{"classic", "modern", "boxed", "compact"} {
		c := cfg("58")
		c.ReceiptStyle = style
		lines := Preview(Document(sampleDetail(), c, Options{}))
		var all []string
		for _, l := range lines {
			for _, r := range l.Text {
				if r < 0x20 {
					t.Fatalf("%s: control byte 0x%x in %q", style, r, l.Text)
				}
			}
			all = append(all, l.Text)
		}
		text := strings.Join(all, "\n")
		for _, want := range []string{"Karots Store", "Onion", "TOTAL", "Karots Business Suite"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s: preview missing %q", style, want)
			}
		}
	}
}
