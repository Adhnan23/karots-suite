package api

import (
	"net/http"

	"karots-suite/internal/features/products"

	"github.com/labstack/echo/v4"
)

// registerLegacySync keeps the two karots-pos paths the Stock Capture phone app
// calls (one-way, read-only catalogue pull over the shop WiFi), so the app works
// against the suite unchanged. Nothing else lives under /api outside /api/v1.
// The app is a native client: it reads the token from the body and sends it
// back as a Bearer header (the browser SPA never sees a token).
func (h *handlers) registerLegacySync(e *echo.Echo, limiter, jwt echo.MiddlewareFunc) {
	e.POST("/api/auth/login", h.LegacySyncLogin, limiter)
	e.GET("/api/sync/catalog", h.LegacySyncCatalog, jwt)
}

func (h *handlers) LegacySyncLogin(c echo.Context) error {
	in, err := h.bindLogin(c)
	if err != nil {
		return err
	}
	pair, err := h.d.Auth.Login(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": map[string]string{"access_token": pair.AccessToken}})
}

// LegacySyncCatalog is every active product with its category path, unit, price
// and on-hand quantity, in karots-pos's field names.
func (h *handlers) LegacySyncCatalog(c echo.Context) error {
	rows, err := products.NewService(h.d.DB).SyncCatalog(c.Request().Context())
	if err != nil {
		return err
	}
	if rows == nil {
		rows = []products.SyncRow{}
	}
	return c.JSON(http.StatusOK, map[string]any{"data": rows})
}
