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
