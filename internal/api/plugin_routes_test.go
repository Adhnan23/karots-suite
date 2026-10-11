package api

import (
	"net/http"
	"testing"

	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// testPluginMux stands in for the plugins a build compiles in: newTestEnv mounts
// it exactly as BuildServer mounts the real one.
var testPluginMux = func() *plugin.Mux {
	m := plugin.NewMux()
	ok := func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"ok": "yes"}) }
	m.Admin().GET("/fixture/admin", ok)
	m.Cashier().GET("/fixture/cashier", ok)
	m.Public().GET("/fixture/public", ok)
	return m
}()

// Plugin routes live under /api/v1/x with the same guards core uses: Admin is
// admin + manager, Cashier is any signed-in, unlocked session, Public is open.
func TestPluginRoutesMountWithCoreGuards(t *testing.T) {
	te := newTestEnv(t)
	cashier := te.login(t, "0770009301", "cashier")
	manager := te.login(t, "0770009302", "manager")
	for _, c := range []struct {
		path   string
		cookie *http.Cookie
		want   int
	}{
		{"/api/v1/x/fixture/admin", nil, http.StatusUnauthorized},
		{"/api/v1/x/fixture/admin", cashier, http.StatusForbidden},
		{"/api/v1/x/fixture/admin", manager, http.StatusOK},
		{"/api/v1/x/fixture/cashier", nil, http.StatusUnauthorized},
		{"/api/v1/x/fixture/cashier", cashier, http.StatusOK},
		{"/api/v1/x/fixture/public", nil, http.StatusOK},
	} {
		if rec := te.do(http.MethodGet, c.path, nil, c.cookie); rec.Code != c.want {
			t.Errorf("%s (cookie %v): %d, want %d — %s", c.path, c.cookie != nil, rec.Code, c.want, rec.Body)
		}
	}
}
