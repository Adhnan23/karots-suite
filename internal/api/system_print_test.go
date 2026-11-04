package api

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"karots-suite/internal/receiptimg"
)

func whitePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A printer on the network that hands back what it was sent.
func catchPrint(t *testing.T) (target string, got <-chan []byte) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	ch := make(chan []byte, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		b, _ := io.ReadAll(c)
		ch <- b
	}()
	return "tcp://" + l.Addr().String(), ch
}

// Appearance prints what it previews, on the printer picked there; nobody else may.
func TestAppearancePrintsOnThePickedPrinter(t *testing.T) {
	te := newTestEnv(t)
	sys := systemLogin(t, te, "0770009017")
	post := func(path string, body []byte, ck *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "image/png")
		req.AddCookie(ck)
		rec := httptest.NewRecorder()
		te.e.ServeHTTP(rec, req)
		return rec
	}

	target, got := catchPrint(t)
	slip := whitePNG(t, receiptimg.PrinterDots("80"), 100)
	if rec := post("/api/v1/system/print/slip?size=80&printer="+url.QueryEscape(target), slip, sys); rec.Code != http.StatusNoContent {
		t.Fatalf("slip %d %s", rec.Code, rec.Body)
	}
	if job := <-got; len(job) == 0 {
		t.Fatal("no slip reached the printer")
	}

	target, got = catchPrint(t)
	if rec := post("/api/v1/system/print/label?size=50x25&printer="+url.QueryEscape(target), whitePNG(t, 400, 200), sys); rec.Code != http.StatusNoContent {
		t.Fatalf("label %d %s", rec.Code, rec.Body)
	}
	if job := string(<-got); !strings.Contains(job, "PRINT 1") || !strings.Contains(job, "SIZE 50") {
		t.Fatalf("label job %.80q", job)
	}

	if rec := te.do(http.MethodGet, "/api/v1/system/printers", nil, sys); rec.Code != http.StatusOK {
		t.Fatalf("printers %d %s", rec.Code, rec.Body)
	}
	admin := te.login(t, devPhone(), "admin")
	if rec := post("/api/v1/system/print/slip", slip, admin); rec.Code != http.StatusNotFound { // system routes don't show themselves
		t.Fatalf("an admin printed from here: %d", rec.Code)
	}
}
