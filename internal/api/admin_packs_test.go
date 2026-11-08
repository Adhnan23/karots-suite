package api

import (
	"fmt"
	"net/http"
	"testing"
)

// The setup app reads every product's packs in one call.
func TestAdminAllPacks(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_packs WHERE product_id = $1`, f.plainID) }) //nolint:errcheck
	admin := te.login(t, "0770009102", "admin")
	path := fmt.Sprintf("/api/v1/admin/products/%d/packs", f.plainID)
	for _, k := range []PackRequest{{Name: "Box", Size: "100"}, {Name: "Card", Size: "10", SellingPrice: "950"}} {
		if rec := te.do(http.MethodPost, path, k, admin); rec.Code >= 300 {
			t.Fatalf("pack %d %s", rec.Code, rec.Body.String())
		}
	}
	all := decode[map[string][]AdminPack](t, te.do(http.MethodGet, "/api/v1/admin/products/packs", nil, admin))
	got := all[fmt.Sprint(f.plainID)]
	if len(got) != 2 || got[0].Name != "Card" || got[0].SellingPrice != "950.00" || got[1].Size != "100" {
		t.Fatalf("smallest first: %+v", all)
	}
	te.db.Exec(`UPDATE products SET is_active = false WHERE id = $1`, f.plainID)                      //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE products SET is_active = true WHERE id = $1`, f.plainID) }) //nolint:errcheck
	if all = decode[map[string][]AdminPack](t, te.do(http.MethodGet, "/api/v1/admin/products/packs", nil, admin)); len(all[fmt.Sprint(f.plainID)]) != 0 {
		t.Fatalf("a disabled product's packs: %+v", all)
	}
}
