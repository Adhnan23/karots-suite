package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Stock-take and the count sheet go batch by batch for a product with 2+ batches (M7.6b).
func TestStockTakeByBatch(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009572"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "admin")
	t.Cleanup(clean)
	te.db.MustExec(`UPDATE settings SET stock_take_enabled=true`)
	code := fmt.Sprintf("TLOT%d", time.Now().UnixNano())
	te.db.MustExec(`UPDATE products SET barcode=$1 WHERE id=$2`, code, f.lotID)

	l := decode[StockTakeList](t, te.do(http.MethodGet, "/api/v1/admin/stock/take?q="+code, nil, ck))
	if len(l.Items) != 1 || len(l.Items[0].Lots) != 2 {
		t.Fatalf("row should carry its 2 batches: %+v", l.Items)
	}
	a, b := l.Items[0].Lots[0], l.Items[0].Lots[1]
	if p := decode[StockTakeList](t, te.do(http.MethodGet, "/api/v1/admin/stock/take?q=T+plain", nil, ck)); len(p.Items) > 0 && p.Items[0].Lots != nil && len(p.Items[0].Lots) > 0 {
		t.Fatalf("a one-batch product carries no batches: %+v", p.Items[0])
	}

	// Batch counts win; the product-level count on the same line is ignored.
	res := decode[StockTakeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/take", StockTakeRequest{Lines: []CountLine{{
		ProductID: f.lotID, Counted: "99",
		Batches:    []BatchCount{{BatchID: a.ID, Counted: "3", Cost: "48", Selling: "95"}, {BatchID: b.ID, Counted: "6"}},
		NewBatches: []AddBatchRequest{{Qty: "2", Cost: "40", Expiry: "2027-06-30"}},
	}}}, ck))
	var qa, qb, qty float64
	te.db.Get(&qa, `SELECT qty_remaining FROM stock_batches WHERE id=$1`, a.ID)  //nolint:errcheck
	te.db.Get(&qb, `SELECT qty_remaining FROM stock_batches WHERE id=$1`, b.ID)  //nolint:errcheck
	te.db.Get(&qty, `SELECT quantity FROM stock WHERE product_id=$1`, f.lotID) //nolint:errcheck
	if res.Saved != 1 || qa != 3 || qb != 6 || qty != 11 {
		t.Fatalf("by batch: saved %d, batches %v / %v, stock %v (want 3 / 6, 11)", res.Saved, qa, qb, qty)
	}
	var ac, ap float64
	te.db.QueryRow(`SELECT cost_price, selling_price FROM stock_batches WHERE id=$1`, a.ID).Scan(&ac, &ap) //nolint:errcheck
	if ac != 48 || ap != 95 {
		t.Fatalf("batch cost / price %v / %v, want 48 / 95", ac, ap)
	}

	// The sheet: one row per batch for this product, batch id in the batch column.
	rec := te.do(http.MethodGet, "/api/v1/admin/stock/take/sheet", nil, ck)
	rows := 0
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, code+",") {
			rows++
		}
	}
	if !strings.Contains(strings.SplitN(rec.Body.String(), "\n", 2)[0], ",batch,expiry") || rows != 3 {
		t.Fatalf("sheet: %d rows for the 3-batch product; header %.120s", rows, rec.Body)
	}

	// Import: a batch row counts that batch (and sets its cost); a row with no
	// batch column at all still counts the product as before.
	upload := func(csv string) ImportSummary {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		fw, _ := mw.CreateFormFile("file", "count.csv")
		fw.Write([]byte(csv)) //nolint:errcheck
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/stock/take/import", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(ck)
		rec := httptest.NewRecorder()
		te.e.ServeHTTP(rec, req)
		return decode[ImportSummary](t, rec)
	}
	ida := strconv.FormatInt(a.ID, 10)
	sum := upload("barcode,counted_qty,cost,batch\n" + code + ",1,44," + ida + "\n" + code + ",1,,999999999\n")
	var ca float64
	te.db.Get(&qa, `SELECT qty_remaining FROM stock_batches WHERE id=$1`, a.ID) //nolint:errcheck
	te.db.Get(&ca, `SELECT cost_price FROM stock_batches WHERE id=$1`, a.ID)    //nolint:errcheck
	if sum.Updated != 1 || sum.Skipped != 1 || qa != 1 || ca != 44 {
		t.Fatalf("import by batch: %+v batch %v @ %v", sum, qa, ca)
	}
	sum = upload("barcode,counted_qty\n" + f.plainBarcode + ",4\n")
	te.db.Get(&qty, `SELECT quantity FROM stock WHERE product_id=$1`, f.plainID) //nolint:errcheck
	if sum.Updated != 1 || qty != 4 {
		t.Fatalf("old sheet: %+v qty %v", sum, qty)
	}
}
