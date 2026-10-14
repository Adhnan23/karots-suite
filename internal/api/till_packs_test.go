package api

import (
	"fmt"
	"net/http"
	"testing"
)

// A cashier asked for "2 cards" of something with no card size sets it at the
// till; the till then sells by the card, a card's own code scans to the card,
// and the item waits on Items to review until someone checks the size.
func TestTillPacks(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	if _, err := te.db.Exec(`UPDATE products SET unit_id = (SELECT id FROM units WHERE NOT allow_decimal ORDER BY id LIMIT 1) WHERE id = $1`, f.plainID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_packs WHERE product_id = $1`, f.plainID) }) //nolint:errcheck
	ck := te.login(t, "0770009101", "cashier")
	path := fmt.Sprintf("/api/v1/till/products/%d/packs", f.plainID)

	rec := te.do(http.MethodPost, path, PackRequest{Name: "Card", Size: "10"}, ck)
	k := decode[TillPack](t, rec)
	if rec.Code != http.StatusCreated || k.Size != "10" || k.SellingPrice != "0.00" {
		t.Fatalf("create %d %+v", rec.Code, k)
	}
	if again := decode[TillPack](t, te.do(http.MethodPost, path, PackRequest{Name: "card", Size: "12"}, ck)); again.ID != k.ID || again.Size != "10" {
		t.Fatalf("same name again %+v", again)
	}
	p := decode[TillProduct](t, te.do(http.MethodGet, "/api/v1/till/products/barcode/"+f.plainBarcode, nil, ck))
	if len(p.Packs) != 1 || p.Packs[0].ID != k.ID || p.ScannedPackID != 0 {
		t.Fatalf("product packs %+v", p)
	}

	admin := te.login(t, "0770009102", "admin")
	rec = te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/products/%d/packs/%d", f.plainID, k.ID),
		PackRequest{Name: "Card", Size: "10", SellingPrice: "950", Barcode: "TEST-PACK-CARD"}, admin)
	if rec.Code != 200 {
		t.Fatalf("admin edit %d %s", rec.Code, rec.Body.String())
	}
	p = decode[TillProduct](t, te.do(http.MethodGet, "/api/v1/till/products/barcode/TEST-PACK-CARD", nil, ck))
	if p.ID != f.plainID || p.ScannedPackID != k.ID || p.Packs[0].SellingPrice != "950.00" {
		t.Fatalf("scan card code %+v", p)
	}

	// Editing in admin counted as checking it; a new till-set size goes back on review.
	te.do(http.MethodPost, path, PackRequest{Name: "Box", Size: "100"}, ck)
	onReview := func() string {
		for _, r := range decode[[]AdminProductRow](t, te.do(http.MethodGet, "/api/v1/admin/products/review", nil, admin)) {
			if r.ID == f.plainID {
				return r.TillPack
			}
		}
		return ""
	}
	if got := onReview(); got != "Box ×100" {
		t.Fatalf("review shows %q", got)
	}
	te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/review-done", f.plainID), nil, admin)
	if got := onReview(); got != "" {
		t.Fatalf("still on review %q", got)
	}
}
