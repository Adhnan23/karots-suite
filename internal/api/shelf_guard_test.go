package api

import (
	"fmt"
	"net/http"
	"testing"

	"karots-suite/internal/plugin"
)

func TestShelfGuard(t *testing.T) {
	te := newTestEnv(t)
	fx := te.seedTill(t)
	ph := te.shelfCashier(t, true)
	ck := sessionCookie(t, te.shelf("dev-S", http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: ph, Pin: "1234", App: shelfApp}, nil))
	get := func(path string, shelf bool) int {
		if shelf {
			return te.shelf("dev-S", http.MethodGet, path, nil, ck).Code
		}
		return te.on("dev-S", http.MethodGet, path, nil, ck).Code
	}
	if c := get("/api/v1/admin/units", true); c != http.StatusOK {
		t.Fatalf("Shelf units: %d", c)
	}
	product := fmt.Sprintf("/api/v1/admin/products/%d", fx.plainID)
	if c := get(product, true); c != http.StatusOK {
		t.Fatalf("Shelf product: %d", c)
	}
	for _, p := range []string{"/api/v1/admin/sales", "/api/v1/admin/settings", "/api/v1/admin/users"} {
		if c := get(p, true); c != http.StatusForbidden {
			t.Fatalf("Shelf reached %s: %d", p, c)
		}
	}
	if c := get("/api/v1/admin/units", false); c != http.StatusForbidden {
		t.Fatalf("a ticked cashier without the Shelf header reached admin: %d", c)
	}
	te.db.Exec(`UPDATE users SET can_use_shelf = false WHERE phone = $1`, ph) //nolint:errcheck
	if r := te.shelf("dev-S", http.MethodGet, "/api/v1/admin/units", nil, ck); r.Code != http.StatusForbidden || errCode(r) != "SHELF_NO_ACCESS" {
		t.Fatalf("unticked mid-session: %d %s", r.Code, r.Body)
	}
	admin := te.login(t, devPhone(), "admin")
	if c := te.do(http.MethodGet, "/api/v1/admin/sales", nil, admin).Code; c != http.StatusOK {
		t.Fatalf("admin without the header: %d", c)
	}
}

func TestShelfPluginRoutes(t *testing.T) {
	t.Cleanup(plugin.AllowShelfForTest("GET", "/t/ping"))
	if !shelfAllowed("GET", "/api/v1/x/t/ping") {
		t.Fatal("a plugin's Shelf route isn't allowed")
	}
	if shelfAllowed("GET", "/api/v1/x/t/other") || shelfAllowed("POST", "/api/v1/x/t/ping") {
		t.Fatal("more than the plugin's Shelf route is allowed")
	}
	if !shelfAllowed("PUT", "/api/v1/admin/products/:id") || shelfAllowed("POST", "/api/v1/admin/products/:id/merge/:into") {
		t.Fatal("core list wrong")
	}
}
