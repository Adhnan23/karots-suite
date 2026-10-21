package api

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"karots-suite/internal/escpos"
)

// fakePrinter points the shop's receipt printer at a raw TCP capture and returns
// the next job it receives (or nil after 3 s).
func fakePrinter(t *testing.T, te *testEnv) func() []byte {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	jobs := make(chan []byte, 8)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			b, _ := io.ReadAll(conn)
			conn.Close()
			jobs <- b
		}
	}()
	var was string
	var kick bool
	te.db.Get(&was, `SELECT receipt_printer FROM settings`)                                                  //nolint:errcheck
	te.db.Get(&kick, `SELECT open_cash_drawer FROM settings`)                                                //nolint:errcheck
	te.db.Exec(`UPDATE settings SET receipt_printer=$1, open_cash_drawer=true`, "tcp://"+ln.Addr().String()) //nolint:errcheck
	t.Cleanup(func() {
		ln.Close()
		te.db.Exec(`UPDATE settings SET receipt_printer=$1, open_cash_drawer=$2`, was, kick) //nolint:errcheck
	})
	return func() []byte {
		select {
		case b := <-jobs:
			return b
		case <-time.After(3 * time.Second):
			return nil
		}
	}
}

func slipPNG(w, h int) []byte {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(255)
			if (x+y)%7 == 0 {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img) //nolint:errcheck
	return b.Bytes()
}

func (te *testEnv) postPNG(path string, body []byte, ck *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/png")
	req.AddCookie(ck)
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func TestPrintSaleImage(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	next := fakePrinter(t, te)

	var cardAcct int64
	te.db.Get(&cardAcct, `INSERT INTO lockers (name, kind) VALUES ($1, 'card') RETURNING id`, fmt.Sprintf("T card %d", time.Now().UnixNano())) //nolint:errcheck
	mk := func(method string) int64 {
		pay := PaymentRequest{Method: method, Amount: "100"}
		if method == "card" {
			pay.AccountID = cardAcct
		}
		req := sale(f.plainID, "1", pay)
		req.DeferDrawerKick = true // auto-print: the kick rides in the print job
		rec := te.do(http.MethodPost, "/api/v1/till/sales", req, ck)
		if rec.Code != 201 {
			t.Fatalf("%s sale: %d %s", method, rec.Code, rec.Body)
		}
		return decode[SaleResult](t, rec).ID
	}
	cash, card := mk("cash"), mk("card")
	pic := slipPNG(576, 300)
	img, _ := png.Decode(bytes.NewReader(pic))
	want, _ := escpos.ImageJob(img, 576)
	kick := []byte{0x1B, 'p'}

	print := func(id int64, q string, body []byte) []byte {
		t.Helper()
		rec := te.postPNG(fmt.Sprintf("/api/v1/till/sales/%d/print%s", id, q), body, ck)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("print %d%s: %d %s", id, q, rec.Code, rec.Body)
		}
		return next()
	}
	if got := print(cash, "?kick=1", pic); !bytes.HasPrefix(got, kick) || !bytes.HasSuffix(got, want) || len(got) != len(want)+5 {
		t.Fatalf("cash + kick: % x…", got[:min(16, len(got))])
	}
	if got := print(cash, "", pic); !bytes.Equal(got, want) {
		t.Fatal("cash reprint: want the image job alone, no kick")
	}
	if got := print(card, "?kick=1", pic); !bytes.Equal(got, want) {
		t.Fatal("card sale never kicks")
	}

	// ?size=58 is checked against 384 dots, not the shop's 80 mm.
	if rec := te.postPNG(fmt.Sprintf("/api/v1/till/sales/%d/print?size=58", cash), pic, ck); rec.Code != http.StatusUnprocessableEntity || !bytes.Contains(rec.Body.Bytes(), []byte("BAD_IMAGE")) {
		t.Fatalf("576 on 58 mm: %d %s", rec.Code, rec.Body)
	}
	if got := print(cash, "?size=58", slipPNG(384, 50)); !bytes.Contains(got, []byte{0x1D, 'v', '0', 0, 48, 0}) {
		t.Fatal("384 on 58 mm: no 48-byte raster rows")
	}

	// No picture: Classic text, as before.
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/sales/%d/print", cash), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("classic: %d", rec.Code)
	} else if got := next(); !bytes.Contains(got, []byte("Karots Business Suite")) {
		t.Fatal("classic print lost its text")
	}

	// The test slip takes a picture too, and keeps its own drawer pulse.
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	if rec := te.postPNG("/api/v1/admin/settings/printer/test", pic, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("test print: %d %s", rec.Code, rec.Body)
	}
	if got := next(); !bytes.Equal(got, want) {
		t.Fatal("test print: want the image job")
	}
}
