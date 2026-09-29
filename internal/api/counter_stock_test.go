package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCounterWriteOffAndConvert(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier") // movements anchor the user
	onHand := func(id int64) string {
		var q string
		te.db.Get(&q, `SELECT quantity::float8::text FROM stock WHERE product_id=$1`, id) //nolint:errcheck
		return q
	}

	// Own use books a different movement than damage, but both take stock out.
	rec := te.do(http.MethodPost, "/api/v1/till/write-off", WriteOffRequest{ProductID: f.plainID, Quantity: "2", Reason: "own_use", Note: "tea"}, ck)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("own use: %d %s", rec.Code, rec.Body)
	}
	var mtype string
	te.db.Get(&mtype, `SELECT type FROM stock_movements WHERE product_id=$1 ORDER BY id DESC LIMIT 1`, f.plainID) //nolint:errcheck
	if onHand(f.plainID) != "8" || mtype != "own_use" {
		t.Fatalf("after own use: stock %s, movement %q", onHand(f.plainID), mtype)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/write-off", WriteOffRequest{ProductID: f.plainID, Quantity: "1", Reason: "gift"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("unknown reason: %d", rec.Code)
	}

	// 1 plain = 2 scarce.
	var cid int64
	if err := te.db.Get(&cid, `INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 2) RETURNING id`, f.plainID, f.scarceID); err != nil {
		t.Fatal(err)
	}
	rows := decode[[]ConversionRow](t, te.do(http.MethodGet, "/api/v1/till/conversions", nil, ck))
	found := false
	for _, r := range rows {
		found = found || (r.ID == cid && r.FromStock == "8" && r.Ratio == "2")
	}
	if !found {
		t.Fatalf("conversion %d not listed with stock: %+v", cid, rows)
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/conversions/%d/run", cid), RunConversionRequest{Quantity: "1"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("run: %d %s", rec.Code, rec.Body)
	}
	if onHand(f.plainID) != "7" || onHand(f.scarceID) != "3" {
		t.Fatalf("after run: plain %s scarce %s", onHand(f.plainID), onHand(f.scarceID))
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/conversions/%d/run", cid), RunConversionRequest{Quantity: "0"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("zero qty: %d", rec.Code)
	}
}
