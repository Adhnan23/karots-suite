package tspl

import (
	"strings"
	"testing"
)

func TestDocumentBasics(t *testing.T) {
	out := string(Document(Input{
		Top:      "Onion 1kg",
		Code:     "4001234567890",
		Format:   "CODE128",
		Bottom:   "Rs. 250.00",
		Count:    3,
		WidthMM:  50,
		HeightMM: 25,
		GapMM:    2,
	}))

	wants := []string{
		"SIZE 50 mm, 25 mm",
		"GAP 2 mm, 0 mm",
		"CLS",
		"\"4001234567890\"", // barcode value
		"\"128\"",           // CODE128 type token
		"Rs. 250.00",        // price line
		"PRINT 3,1",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n---\n%s", w, out)
		}
	}
}

func TestDocumentDefaultsAndFormats(t *testing.T) {
	// Zero sizes/count fall back to sane defaults.
	out := string(Document(Input{Top: "x", Code: "123"}))
	if !strings.Contains(out, "SIZE 50 mm, 25 mm") {
		t.Errorf("expected default 50x25 size, got:\n%s", out)
	}
	if !strings.Contains(out, "PRINT 1,1") {
		t.Errorf("expected default count 1, got:\n%s", out)
	}

	cases := map[string]string{
		"EAN13":   "EAN13",
		"ean8":    "EAN8",
		"upc":     "UPCA",
		"CODE39":  "39",
		"unknown": "128",
	}
	for format, token := range cases {
		out := string(Document(Input{Code: "1", Format: format, Count: 1}))
		if !strings.Contains(out, "\""+token+"\"") {
			t.Errorf("format %q: expected type %q in:\n%s", format, token, out)
		}
	}
}

func TestWrapLabelText(t *testing.T) {
	// Short name: one line, unchanged.
	if got := wrapLabelText("Onion 1kg", 24, 2); len(got) != 1 || got[0] != "Onion 1kg" {
		t.Errorf("short name should stay one line, got %q", got)
	}
	// Long name wraps onto a second line at a word boundary, each line within width.
	got := wrapLabelText("Front Left Brake Disc Rotor Assembly", 14, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(got), got)
	}
	for i, ln := range got {
		if len(ln) > 14 {
			t.Errorf("line %d over width: %q (%d)", i, ln, len(ln))
		}
	}
	// A single word longer than the whole 2-line budget is hard-split, never overflows.
	got = wrapLabelText("SUPERCALIFRAGILISTICEXPIALIDOCIOUS", 10, 2)
	if len(got) != 2 || len(got[0]) != 10 || len(got[1]) != 10 {
		t.Errorf("over-long word should hard-split to 2 full lines, got %q", got)
	}
}

func TestTopLineCount(t *testing.T) {
	// 40mm wide => (320-32)/12 = 24 chars/line. A long name with the default
	// (NameLines 0 → 1) stays a single font-"2" line; NameLines 2 wraps to two.
	long := "Front Left Brake Disc Rotor Assembly Kit"
	if n := strings.Count(string(Document(Input{Top: long, Code: "1", Count: 1, WidthMM: 40, HeightMM: 30})), "\"2\",0,1,1,"); n != 1 {
		t.Errorf("default should emit 1 top line, got %d", n)
	}
	if n := strings.Count(string(Document(Input{Top: long, NameLines: 2, Code: "1", Count: 1, WidthMM: 40, HeightMM: 30})), "\"2\",0,1,1,"); n != 2 {
		t.Errorf("NameLines=2 should emit 2 top lines, got %d", n)
	}
	// Bottom slot wraps independently on BottomLines.
	if n := strings.Count(string(Document(Input{Bottom: long, BottomLines: 2, Code: "1", Count: 1, WidthMM: 40, HeightMM: 30})), "\"3\",0,1,1,"); n != 2 {
		t.Errorf("BottomLines=2 should emit 2 bottom lines, got %d", n)
	}
}

func TestFontSelection(t *testing.T) {
	// Default (blank) keeps top font "2" / bottom font "3".
	def := string(Document(Input{Top: "A", Bottom: "B", Code: "1", Count: 1}))
	if !strings.Contains(def, "\"2\",0,1,1,\"A\"") || !strings.Contains(def, "\"3\",0,1,1,\"B\"") {
		t.Errorf("default fonts wrong:\n%s", def)
	}
	// Explicit fonts flow through to the TEXT command's font token.
	out := string(Document(Input{Top: "A", Bottom: "B", NameFont: "1", BottomFont: "4", Code: "1", Count: 1}))
	if !strings.Contains(out, "\"1\",0,1,1,\"A\"") || !strings.Contains(out, "\"4\",0,1,1,\"B\"") {
		t.Errorf("explicit fonts not applied:\n%s", out)
	}
}

func TestBottomOmittedWhenBlank(t *testing.T) {
	out := string(Document(Input{Code: "1", Bottom: "", Count: 1}))
	if strings.Contains(out, "TEXT") && strings.Contains(out, "Rs.") {
		t.Errorf("a blank bottom line should print nothing:\n%s", out)
	}
}

func TestAsciiSanitisesAndEscapes(t *testing.T) {
	out := string(Document(Input{Top: "Coke \"500\" අ", Code: "1", Count: 1}))
	if strings.Contains(out, "අ") {
		t.Errorf("non-ASCII rune should be replaced:\n%s", out)
	}
	if !strings.Contains(out, "\\\"500\\\"") {
		t.Errorf("embedded quotes should be escaped:\n%s", out)
	}
}

// A text-only label (a dose sticker, a shelf note): left-aligned lines, no
// barcode; a long line wraps, and lines that would fall off the label are dropped.
func TestTextLabel(t *testing.T) {
	in := Input{Lines: []string{"Nimal Perera  14 Oct", "Metfor 500mg x 60", "1-0-1  Morning, Night", "30 days - After meals"}, Count: 2, WidthMM: 40, HeightMM: 30}
	out := string(Document(in))
	if strings.Contains(out, "BARCODE") {
		t.Fatalf("a text label has no barcode:\n%s", out)
	}
	for _, w := range []string{"Nimal Perera", "Metfor 500mg x 60", "Morning, Night", "After meals", "PRINT 2,1"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q\n%s", w, out)
		}
	}
	l := LayoutOf(in)
	for i, tx := range l.Texts {
		if tx.Y+tx.LineH > l.HeightDots || tx.X+len(tx.Value)*tx.CharW > l.WidthDots {
			t.Errorf("line %d off the label: %+v", i, tx)
		}
		if i > 0 && tx.Y <= l.Texts[i-1].Y {
			t.Errorf("lines must go down the label: %+v", l.Texts)
		}
	}
	many := LayoutOf(Input{Lines: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, WidthMM: 40, HeightMM: 30})
	if last := many.Texts[len(many.Texts)-1]; last.Y+last.LineH > many.HeightDots {
		t.Fatalf("an overflowing line was kept: %+v", last)
	}
	if !many.Cut || l.Cut {
		t.Fatalf("Cut says whether lines were dropped: many %v, fits %v", many.Cut, l.Cut)
	}
}

// Common typography prints as its ASCII look-alike instead of "?".
func TestASCIILookAlikes(t *testing.T) {
	if got := ascii("½-0-½ × 2 · Morning – Night — ‘x’ “y”"); got != "1/2-0-1/2 x 2 - Morning - Night - 'x' \"y\"" {
		t.Fatalf("got %q", got)
	}
	if got := ascii("සිංහල"); got != "?????" {
		t.Fatalf("other scripts still show as ?: %q", got)
	}
}
