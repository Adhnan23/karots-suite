package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// A new product can carry its packs and extra barcodes in one save.
func TestCreateWithPacksAndBarcodes(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009531", "admin")
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	sfx := time.Now().UnixNano()
	code := fmt.Sprintf("X%d", sfx)
	req := ProductRequest{Name: fmt.Sprintf("T pack new %d", sfx), CategoryID: cat, UnitID: 1, SellingPrice: "5",
		Packs: []PackRequest{{Name: "Card", Size: "10"}}, Barcodes: []string{code}}
	rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	id := decode[ProductSaved](t, rec).ID
	t.Cleanup(func() { te.db.Exec(`DELETE FROM products WHERE id=$1`, id) }) //nolint:errcheck
	packs := decode[[]AdminPack](t, te.do(http.MethodGet, "/api/v1/admin/products/"+strconv.FormatInt(id, 10)+"/packs", nil, ck))
	if len(packs) != 1 || packs[0].Name != "Card" {
		t.Fatalf("packs: %+v", packs)
	}
	var owner int64
	te.db.Get(&owner, `SELECT product_id FROM product_barcodes WHERE barcode=$1`, code) //nolint:errcheck
	if owner != id {
		t.Fatalf("barcode owner %d, want %d", owner, id)
	}
}

// A pack that can't be saved refuses the whole form before anything is written.
func TestCreateRefusesBadPackBeforeWriting(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009532", "admin")
	var cat, decimalUnit int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID)  //nolint:errcheck
	te.db.Get(&decimalUnit, `SELECT id FROM units WHERE allow_decimal LIMIT 1`) //nolint:errcheck
	if decimalUnit == 0 {
		t.Fatal("seed has no decimal unit")
	}
	sfx := time.Now().UnixNano()
	for name, req := range map[string]ProductRequest{
		"size 1":       {UnitID: 1, Packs: []PackRequest{{Name: "Card", Size: "1"}}},
		"no name":      {UnitID: 1, Packs: []PackRequest{{Name: " ", Size: "10"}}},
		"same twice":   {UnitID: 1, Packs: []PackRequest{{Name: "Card", Size: "10"}, {Name: "card", Size: "12"}}},
		"decimal unit": {UnitID: decimalUnit, Packs: []PackRequest{{Name: "Card", Size: "10"}}},
	} {
		req.Name, req.CategoryID = fmt.Sprintf("T pack bad %d %s", sfx, name), cat
		if rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck); rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
		var n int
		te.db.Get(&n, `SELECT count(*) FROM products WHERE name=$1`, req.Name) //nolint:errcheck
		if n != 0 {
			t.Errorf("%s: product was written", name)
		}
	}
}

// A barcode another product owns only shows on the database's side: the new
// product is kept, and the message says what didn't save.
func TestCreateBarcodeTakenKeepsProduct(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009533", "admin")
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	sfx := time.Now().UnixNano()
	code := fmt.Sprintf("Y%d", sfx)
	if _, err := te.db.Exec(`UPDATE products SET barcode=$1 WHERE id=$2`, code, f.plainID); err != nil {
		t.Fatal(err)
	}
	rec := te.do(http.MethodPost, "/api/v1/admin/products", ProductRequest{Name: fmt.Sprintf("T pack taken %d", sfx), CategoryID: cat, UnitID: 1, Barcodes: []string{code}}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	saved := decode[ProductSaved](t, rec)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM products WHERE id=$1`, saved.ID) }) //nolint:errcheck
	if saved.Message == "Product added" {
		t.Fatalf("message hides the failed barcode: %q", saved.Message)
	}
}
