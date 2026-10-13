package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Extra barcodes through the admin API: a cashier can't touch them, a taken code
// is a 409 naming its product, Make main swaps, the list and rows carry them.
func TestAdminBarcodes(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009591", "manager")
	cashier := te.login(t, "0770009592", "cashier")
	path := "/api/v1/admin/products/" + strconv.FormatInt(f.plainID, 10) + "/barcodes"
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_barcodes WHERE product_id = $1`, f.plainID) }) //nolint:errcheck

	if rec := te.do(http.MethodPost, path, BarcodeRequest{Barcode: "TEST-API-X", Source: "form"}, cashier); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier add: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, path, BarcodeRequest{Barcode: "TEST-API-X", Source: "form"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", rec.Code, rec.Body)
	}
	var other int64
	te.db.Get(&other, `SELECT id FROM products WHERE id <> $1 AND is_active LIMIT 1`, f.plainID) //nolint:errcheck
	rec := te.do(http.MethodPost, "/api/v1/admin/products/"+strconv.FormatInt(other, 10)+"/barcodes", BarcodeRequest{Barcode: "TEST-API-X", Source: "form"}, ck)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "T plain") {
		t.Fatalf("taken: %d %s", rec.Code, rec.Body)
	}
	row := decode[AdminProduct](t, te.do(http.MethodGet, "/api/v1/admin/products/"+strconv.FormatInt(f.plainID, 10), nil, ck))
	if len(row.Barcodes) != 1 || row.Barcodes[0] != "TEST-API-X" {
		t.Fatalf("row barcodes %v", row.Barcodes)
	}
	if rec := te.do(http.MethodPost, path+"/TEST-API-X/main", nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("make main: %d %s", rec.Code, rec.Body)
	}
	list := decode[[]ProductBarcode](t, te.do(http.MethodGet, path, nil, ck))
	if len(list) != 2 || !list[0].Main || list[0].Barcode != "TEST-API-X" || list[1].Barcode != f.plainBarcode {
		t.Fatalf("list after make main %+v", list)
	}
	// Put the fixture back as it was: its barcode main again, then drop the extra.
	if rec := te.do(http.MethodPost, path+"/"+f.plainBarcode+"/main", nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("make main back: %d", rec.Code)
	}
	if rec := te.do(http.MethodDelete, path+"/TEST-API-X", nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodDelete, path+"/TEST-API-X", nil, ck); rec.Code != http.StatusNotFound {
		t.Fatalf("remove twice: %d", rec.Code)
	}
}
