package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminStockTake(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009571"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "admin")
	t.Cleanup(clean)
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET stock_take_enabled=true`) }) //nolint:errcheck

	te.db.Exec(`UPDATE settings SET stock_take_enabled=false`) //nolint:errcheck
	if rec := te.do(http.MethodGet, "/api/v1/admin/stock/take", nil, ck); rec.Code != http.StatusForbidden {
		t.Fatalf("toggle off: %d", rec.Code)
	}
	te.db.Exec(`UPDATE settings SET stock_take_enabled=true`) //nolint:errcheck

	l := decode[StockTakeList](t, te.do(http.MethodGet, "/api/v1/admin/stock/take?q=T+lot", nil, ck))
	if l.Total < 1 || !l.Items[0].MultiPrice {
		t.Fatalf("list flags the two-price product: %+v", l.Items)
	}

	res := decode[StockTakeResult](t, te.do(http.MethodPost, "/api/v1/admin/stock/take", StockTakeRequest{Lines: []CountLine{
		{ProductID: f.plainID, Counted: "7", Cost: "55"},
		{ProductID: f.scarceID, Counted: "1"}, // already 1: unchanged
	}}, ck))
	var qty, cost float64
	te.db.Get(&qty, `SELECT quantity FROM stock WHERE product_id=$1`, f.plainID) //nolint:errcheck
	te.db.Get(&cost, `SELECT cost_price FROM products WHERE id=$1`, f.plainID)   //nolint:errcheck
	if res.Saved != 1 || qty != 7 || cost != 55 {
		t.Fatalf("apply: saved %d qty %v cost %v", res.Saved, qty, cost)
	}

	if rec := te.do(http.MethodGet, "/api/v1/admin/stock/take/sheet", nil, ck); rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "barcode,name,current_qty,counted_qty") {
		t.Fatalf("sheet: %d %.80s", rec.Code, rec.Body)
	}

	// Upload: one good row, one unknown barcode, one blank count.
	csv := "Code,Product,Count,Selling Price\n" + f.plainBarcode + ",x,9,110\n999999999,y,1,\n" + f.plainBarcode + ",x,,\n"
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
	sum := decode[ImportSummary](t, rec)
	te.db.Get(&qty, `SELECT quantity FROM stock WHERE product_id=$1`, f.plainID) //nolint:errcheck
	if sum.Updated != 1 || sum.Skipped != 1 || len(sum.Errors) != 1 || sum.Errors[0].Line != 3 || qty != 9 {
		t.Fatalf("import: %+v qty %v", sum, qty)
	}
}
