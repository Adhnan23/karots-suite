package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// A barcode a cashier links works at once and waits on Items to review; an admin's doesn't.
func TestCashiersLinkedBarcodeWaitsForReview(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	cashier := te.login(t, devPhone(), "cashier")
	time.Sleep(time.Millisecond)
	boss := te.login(t, devPhone(), "admin")
	code := fmt.Sprintf("88%011d", time.Now().UnixNano()%100_000_000_000)
	link := fmt.Sprintf("/api/v1/till/products/%d/barcodes", f.plainID)
	review := func() *AdminProductRow {
		for _, r := range decode[[]AdminProductRow](t, te.do(http.MethodGet, "/api/v1/admin/products/review", nil, boss)) {
			if r.ID == f.plainID {
				return &r
			}
		}
		return nil
	}

	if rec := te.do(http.MethodPost, link, TillBarcodeRequest{Barcode: code}, cashier); rec.Code >= 300 {
		t.Fatalf("link %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/"+code, nil, cashier); rec.Code != http.StatusOK {
		t.Fatalf("the code doesn't scan yet: %d", rec.Code)
	}
	row := review()
	if row == nil || len(row.TillBarcodes) != 1 || row.TillBarcodes[0].Barcode != code || row.TillBarcodes[0].By == "" || row.TillBarcodes[0].StockSince {
		t.Fatalf("review row %+v", row)
	}

	// Stock received after the link: the admin is warned before removing it.
	te.db.MustExec(`INSERT INTO stock_movements (product_id, type, quantity, user_id, created_at)
		SELECT $1, 'purchase', 5, id, now() + interval '1 minute' FROM users LIMIT 1`, f.plainID)
	if row = review(); row == nil || !row.TillBarcodes[0].StockSince {
		t.Fatalf("no stock warning: %+v", row)
	}

	// Looks fine: off the list, and the code stays.
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/review-done", f.plainID), nil, boss); rec.Code >= 300 {
		t.Fatalf("review-done %d", rec.Code)
	}
	if row := review(); row != nil {
		t.Fatalf("still on the list: %+v", row)
	}

	// An admin's own link isn't reviewed.
	other := code + "1"
	if rec := te.do(http.MethodPost, link, TillBarcodeRequest{Barcode: other, Source: "receive"}, boss); rec.Code >= 300 {
		t.Fatalf("admin link %d %s", rec.Code, rec.Body)
	}
	if row := review(); row != nil {
		t.Fatalf("an admin's link went to review: %+v", row)
	}

	// Removed: the code is unknown again, and so off the list.
	third := code + "2"
	te.do(http.MethodPost, link, TillBarcodeRequest{Barcode: third}, cashier)
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/products/%d/barcodes/%s", f.plainID, third), nil, boss); rec.Code >= 300 {
		t.Fatalf("remove %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/"+third, nil, cashier); rec.Code != http.StatusNotFound {
		t.Fatalf("a removed code still scans: %d", rec.Code)
	}
	if row := review(); row != nil {
		t.Fatalf("a removed code is still listed: %+v", row)
	}
}
