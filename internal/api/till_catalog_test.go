package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
)

func TestTillNeedsSession(t *testing.T) {
	te := newTestEnv(t)
	if rec := te.do(http.MethodGet, "/api/v1/till/config", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", rec.Code)
	}
}

func TestTillCatalog(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009101", "cashier")

	cfg := decode[TillConfig](t, te.do(http.MethodGet, "/api/v1/till/config", nil, ck))
	if cfg.Symbol == "" || cfg.TenderMethods == nil || cfg.IsManager || cfg.CanSeeCost {
		t.Fatalf("config %+v", cfg)
	}

	rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/"+f.plainBarcode, nil, ck)
	if p := decode[TillProduct](t, rec); rec.Code != 200 || p.ID != f.plainID || p.SellingPrice != "100.00" || p.Badges == nil {
		t.Fatalf("barcode %d %+v", rec.Code, p)
	}
	if rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/000000000", nil, ck); rec.Code != 404 {
		t.Fatalf("unknown barcode %d", rec.Code)
	}

	page := decode[ProductPage](t, te.do(http.MethodGet, "/api/v1/till/products?search="+f.plainBarcode, nil, ck))
	if len(page.Items) != 1 || page.Items[0].ID != f.plainID {
		t.Fatalf("search %+v", page)
	}

	top := decode[[]TillGroup](t, te.do(http.MethodGet, "/api/v1/till/groups", nil, ck))
	found := false
	for _, g := range top {
		found = found || g.ID == f.groupID
	}
	gv := decode[GroupView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/groups/%d", f.groupID), nil, ck))
	if !found || len(gv.Products) != 1 || gv.Products[0].ID != f.plainID || gv.Breadcrumb == nil || gv.Children == nil {
		t.Fatalf("groups top-found=%v view=%+v", found, gv)
	}

	opts := decode[map[string][]Lot](t, te.do(http.MethodGet, "/api/v1/till/price-options", nil, ck))
	lots := opts[strconv.FormatInt(f.lotID, 10)]
	if len(lots) != 2 || lots[0].Price == lots[1].Price {
		t.Fatalf("lot product options %+v", lots)
	}
	if _, ok := opts[strconv.FormatInt(f.expiringID, 10)]; !ok {
		t.Fatal("expired-first product must be offered (expiry prompt)")
	}
	if _, ok := opts[strconv.FormatInt(f.plainID, 10)]; ok {
		t.Fatal("single-price, unexpired product must not prompt")
	}
	own := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.expiringID), nil, ck))
	if len(own) != 2 || own[0].ExpiryDate == "" {
		t.Fatalf("lots %+v", own)
	}

	d := decode[ProductDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/detail", f.plainID), nil, ck))
	if d.ShowCost || d.CostPrice != "" {
		t.Fatalf("cashier without flag must not see cost: %+v", d)
	}
	admin := te.login(t, "0770009102", "admin")
	d = decode[ProductDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/detail", f.plainID), nil, admin))
	if !d.ShowCost || d.CostPrice != "50.00" {
		t.Fatalf("admin sees cost: %+v", d)
	}
	if units := decode[[]Unit](t, te.do(http.MethodGet, "/api/v1/till/units", nil, ck)); len(units) == 0 {
		t.Fatal("units empty")
	}
	if picks := te.do(http.MethodGet, "/api/v1/till/quick-picks", nil, ck); picks.Code != 200 {
		t.Fatalf("quick picks %d", picks.Code)
	}
}

// A scan of an extra barcode finds its product and stamps when that code was last
// seen (the product form shows it, so an owner knows which old codes still sell).
func TestTillScanOfExtraBarcodeStampsIt(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009101", "cashier")
	te.db.MustExec(`INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, 'TEST-TILL-EXTRA', 'form')`, f.plainID)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_barcodes WHERE barcode = 'TEST-TILL-EXTRA'`) }) //nolint:errcheck
	rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/TEST-TILL-EXTRA", nil, ck)
	if p := decode[TillProduct](t, rec); rec.Code != 200 || p.ID != f.plainID {
		t.Fatalf("extra scan %d %+v", rec.Code, p)
	}
	var stamped bool
	if err := te.db.Get(&stamped, `SELECT last_scanned_at IS NOT NULL FROM product_barcodes WHERE barcode = 'TEST-TILL-EXTRA'`); err != nil || !stamped {
		t.Fatalf("last scan not stamped: %v", err)
	}
}
