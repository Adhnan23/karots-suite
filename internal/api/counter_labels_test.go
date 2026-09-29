package api

import (
	"bytes"
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
