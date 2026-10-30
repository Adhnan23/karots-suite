package pocketcashier

import (
	"net/http"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// The CA for a phone to trust: .crt (DER, Android), .pem, .mobileconfig (iPhone).
func (h handlers) caFile(kind string) echo.HandlerFunc {
	return func(c echo.Context) error { return h.serveCA(c, kind) }
}

func (h handlers) serveCA(c echo.Context, kind string) error {
	ctx := c.Request().Context()
	ca, _, err := h.p.store.CA(ctx)
	if err != nil {
		return apperr.Internal("failed to read the shop certificate", err)
	}
	c.Response().Header().Set("Cache-Control", "no-cache")
	switch kind {
	case "ca.crt":
		c.Response().Header().Set("Content-Disposition", `attachment; filename="karots-shop-ca.crt"`)
		return c.Blob(http.StatusOK, "application/x-x509-ca-cert", ca.Raw)
	case "ca.pem":
		c.Response().Header().Set("Content-Disposition", `attachment; filename="karots-shop-ca.pem"`)
		return c.Blob(http.StatusOK, "application/x-pem-file", pemOf(ca.Raw))
	case "ca.mobileconfig":
		var shop string
		_ = h.p.core.DB.GetContext(ctx, &shop, `SELECT shop_name FROM settings WHERE id = 1`)
		c.Response().Header().Set("Content-Disposition", `attachment; filename="karots-shop.mobileconfig"`)
		return c.Blob(http.StatusOK, "application/x-apple-aspen-config", []byte(mobileconfig(ca.Raw, shop)))
	}
	return apperr.NotFound("file")
}

// ping answers anyone (the setup page asks from another origin to learn
// whether this phone trusts the certificate yet).
func (h handlers) ping(c echo.Context) error {
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	return c.NoContent(http.StatusNoContent)
}
