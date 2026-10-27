package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

func TestCounterLabels(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009402", "cashier")

	// A fake raw network printer: capture what the till sends.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		b, _ := io.ReadAll(conn)
		conn.Close()
		got <- b
	}()
	var was string
	te.db.Get(&was, `SELECT label_printer FROM settings`)                           //nolint:errcheck
	te.db.Exec(`UPDATE settings SET label_printer=$1`, "tcp://"+ln.Addr().String()) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET label_printer=$1`, was) })   //nolint:errcheck

	rec := te.do(http.MethodPost, "/api/v1/till/labels", LabelRequest{ProductID: f.plainID, Top: "Plain", Bottom: "Rs 100", Count: 3, Size: "40x30"}, ck)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("print: %d %s", rec.Code, rec.Body)
	}
	doc := <-got
	for _, want := range []string{f.plainBarcode, "SIZE 40 mm, 30 mm", "PRINT 3", "Plain"} {
		if !bytes.Contains(doc, []byte(want)) {
			t.Fatalf("label stream lacks %q:\n%s", want, doc)
		}
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/labels", LabelRequest{Count: 1}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("no product, no code: %d", rec.Code)
	}
}

func TestCounterLabelPreviewMatchesPrint(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009403", "cashier")
	l := decode[LabelLayout](t, te.do(http.MethodPost, "/api/v1/till/labels/preview", LabelRequest{ProductID: f.plainID, Top: "Plain", Bottom: "Rs 100", Size: "40x30"}, ck))
	if l.WidthDots != 320 || l.HeightDots != 240 || len(l.Texts) != 2 || l.Barcode.Code != f.plainBarcode {
		t.Fatalf("layout: %+v", l)
	}
	if l.Texts[0].Y >= l.Barcode.Y || l.Texts[1].Y <= l.Barcode.Y {
		t.Fatalf("top above, bottom below the bars: %+v", l)
	}
	// Settings previews its unsaved lines / sizes: a look overrides the saved ones.
	long := "A very long product name that needs more than one line on the label"
	one := decode[LabelLayout](t, te.do(http.MethodPost, "/api/v1/till/labels/preview", LabelRequest{Code: "123", Top: long, Size: "40x30"}, ck))
	three := decode[LabelLayout](t, te.do(http.MethodPost, "/api/v1/till/labels/preview", LabelRequest{Code: "123", Top: long, Size: "40x30",
		Look: &LabelLook{FirstLines: 3, SecondLines: 1, FirstSize: "1", SecondSize: "4"}}, ck))
	if len(three.Texts) <= len(one.Texts) || three.Texts[0].CharW >= one.Texts[0].CharW {
		t.Fatalf("look ignored: %+v vs %+v", three.Texts, one.Texts)
	}
	// Nothing picked yet still previews (an empty label), but never prints.
	if rec := te.do(http.MethodPost, "/api/v1/till/labels/preview", LabelRequest{}, ck); rec.Code != http.StatusOK {
		t.Fatalf("empty preview: %d", rec.Code)
	}
	if rows := decode[[]DetailRow](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/label-fields", f.plainID), nil, ck)); rows == nil {
		t.Fatal("label fields must be [] not null")
	}
}

// A text-only label (a plugin's dose sticker, a shelf note) goes to the same
// label printer at the shop's label size, with no barcode.
func TestCounterTextLabel(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009402", "cashier")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		b, _ := io.ReadAll(conn)
		conn.Close()
		got <- b
	}()
	var was string
	te.db.Get(&was, `SELECT label_printer FROM settings`)                           //nolint:errcheck
	te.db.Exec(`UPDATE settings SET label_printer=$1`, "tcp://"+ln.Addr().String()) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET label_printer=$1`, was) })   //nolint:errcheck

	rec := te.do(http.MethodPost, "/api/v1/till/labels/text", TextLabelRequest{Lines: []string{"Metfor 500mg x 60", "1-0-1  Morning, Night"}, Count: 1, Size: "40x30"}, ck)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("print: %d %s", rec.Code, rec.Body)
	}
	doc := <-got
	if !bytes.Contains(doc, []byte("Morning, Night")) || bytes.Contains(doc, []byte("BARCODE")) {
		t.Fatalf("text label:\n%s", doc)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/labels/text", TextLabelRequest{Lines: []string{" "}}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatal(fmt.Sprintf("blank lines must be refused: %d", rec.Code))
	}
}

// The text label's preview is the print's own layout: its lines, no barcode, and
// whether anything was cut off the bottom.
func TestCounterTextLabelPreview(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009402", "cashier")
	rec := te.do(http.MethodPost, "/api/v1/till/labels/text/preview", TextLabelRequest{Lines: []string{"Metfor 500mg x 60", "1-0-1  Morning · Night"}, Size: "40x30"}, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body)
	}
	var l LabelLayout
	if err := json.Unmarshal(rec.Body.Bytes(), &l); err != nil {
		t.Fatal(err)
	}
	if len(l.Texts) != 2 || l.Texts[1].Value != "1-0-1 Morning - Night" || l.Barcode.Code != "" || l.Cut || l.WidthMM != 40 {
		t.Fatalf("layout: %+v", l)
	}
	long := make([]string, 12)
	for i := range long {
		long[i] = "line"
	}
	rec = te.do(http.MethodPost, "/api/v1/till/labels/text/preview", TextLabelRequest{Lines: long, Size: "40x30"}, ck)
	if json.Unmarshal(rec.Body.Bytes(), &l); !l.Cut {
		t.Fatalf("12 lines on 40x30 must say cut: %+v", l)
	}
}

// Image mode: the SPA sends the label as a picture; the server prints it as one
// TSPL bitmap, count times, on the same label printer — or refuses a wrong size.
func TestLabelImagePrint(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009404", "cashier")
	admin := te.login(t, "0770009405", "admin")
	listen := func() chan []byte {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		got := make(chan []byte, 1)
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			b, _ := io.ReadAll(conn)
			conn.Close()
			got <- b
		}()
		te.db.Exec(`UPDATE settings SET label_printer=$1`, "tcp://"+ln.Addr().String()) //nolint:errcheck
		return got
	}
	var was string
	te.db.Get(&was, `SELECT label_printer FROM settings`)                         //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET label_printer=$1`, was) }) //nolint:errcheck

	for _, c := range []struct {
		path string
		ck   *http.Cookie
		w, h int
		size string
	}{
		{"/api/v1/till/labels?count=2&size=50x25", ck, 400, 200, "SIZE 50 mm, 25 mm"},
		{"/api/v1/admin/labels/send?count=2&size=38x25", admin, 304, 200, "SIZE 38 mm, 25 mm"},
		{"/api/v1/till/labels/text?count=2&size=40x30", ck, 320, 240, "SIZE 40 mm, 30 mm"},
	} {
		got := listen()
		if rec := te.postPNG(c.path, slipPNG(c.w, c.h), c.ck); rec.Code != http.StatusNoContent {
			t.Fatalf("%s: %d %s", c.path, rec.Code, rec.Body)
		}
		doc := <-got
		if !bytes.Contains(doc, []byte(c.size)) || !bytes.Contains(doc, []byte("BITMAP 0,0,")) || !bytes.Contains(doc, []byte("PRINT 2,1")) ||
			bytes.Contains(doc, []byte("TEXT ")) {
			t.Fatalf("%s: job %q…", c.path, doc[:min(len(doc), 120)])
		}
	}
	// A picture of the wrong size (a receipt, or the wrong label) is refused, never printed.
	rec := te.postPNG("/api/v1/till/labels?count=1&size=50x25", slipPNG(576, 200), ck)
	if rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte("BAD_IMAGE")) {
		t.Fatalf("wrong size: %d %s", rec.Code, rec.Body)
	}
	// An unreachable printer fails the picture the way it fails Classic.
	te.db.Exec(`UPDATE settings SET label_printer='tcp://127.0.0.1:1'`) //nolint:errcheck
	img := te.postPNG("/api/v1/till/labels?count=1&size=50x25", slipPNG(400, 200), ck)
	txt := te.do(http.MethodPost, "/api/v1/till/labels", LabelRequest{Code: "12345", Count: 1, Size: "50x25"}, ck)
	if img.Code != txt.Code || img.Code < 500 {
		t.Fatalf("no printer: image %d, classic %d", img.Code, txt.Code)
	}
}
