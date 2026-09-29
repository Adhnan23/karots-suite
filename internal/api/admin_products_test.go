package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestAdminProducts(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009521", "admin")
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	name := fmt.Sprintf("T admin product %d", time.Now().UnixNano())

	req := ProductRequest{Name: name, CategoryID: cat, UnitID: 1, CostPrice: "40", SellingPrice: "65.5", ReorderLevel: 3}
	rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	id := decode[ProductSaved](t, rec).ID
	t.Cleanup(func() { te.db.Exec(`DELETE FROM products WHERE id=$1`, id) }) //nolint:errcheck
	path := "/api/v1/admin/products/" + strconv.FormatInt(id, 10)

	got := decode[AdminProduct](t, te.do(http.MethodGet, path, nil, ck))
	if got.SellingPrice != "65.50" || got.UnitID != 1 || !got.Low || !got.IsActive {
		t.Fatalf("read back: %+v", got)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/products", ProductRequest{Name: name, CategoryID: cat, UnitID: 1, DupSourceName: name}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unrenamed duplicate: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/products", ProductRequest{Name: "x", UnitID: 1}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no category: %d", rec.Code)
	}

	list := decode[ProductList](t, te.do(http.MethodGet, "/api/v1/admin/products?low=1&q="+url.QueryEscape(name)+"&category="+strconv.FormatInt(cat, 10), nil, ck))
	if list.Total < 1 || len(list.Items) == 0 || list.Items[0].ID != id {
		t.Fatalf("filtered list: %+v", list)
	}

	// Disable hides it; ?disabled=1 shows it again; activate brings it back.
	if rec := te.do(http.MethodDelete, path, nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("disable: %d", rec.Code)
	}
	q := "/api/v1/admin/products?q=" + url.QueryEscape(name)
	if l := decode[ProductList](t, te.do(http.MethodGet, q, nil, ck)); l.Total != 0 {
		t.Fatalf("disabled still listed: %+v", l)
	}
	if l := decode[ProductList](t, te.do(http.MethodGet, q+"&disabled=1", nil, ck)); l.Total != 1 || l.Items[0].IsActive {
		t.Fatalf("disabled filter: %+v", l)
	}
	te.do(http.MethodPost, path+"/activate", nil, ck)

	// Finishing a quick-added item clears review and back-fills cost on past sale lines.
	te.db.Exec(`UPDATE products SET needs_review=true WHERE id=$1`, id) //nolint:errcheck
	if n := decode[Count](t, te.do(http.MethodGet, "/api/v1/admin/products/review/count", nil, ck)); n.Count < 1 {
		t.Fatalf("review count: %+v", n)
	}
	req.CostPrice = "42"
	saved := decode[ProductSaved](t, te.do(http.MethodPut, path, req, ck))
	var review bool
	te.db.Get(&review, `SELECT needs_review FROM products WHERE id=$1`, id) //nolint:errcheck
	if review || saved.Message == "Product saved" {
		t.Fatalf("finish review: %v %+v", review, saved)
	}
}
