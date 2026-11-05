package pocketcashier

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// copyFor sends the till copy to a phone: 304 when its ETag is still the
// version, else the whole copy (gzipped when the phone accepts it).
func (h handlers) copyFor(c echo.Context) error {
	ctx := c.Request().Context()
	p := phoneFrom(c)
	v, err := h.p.core.TillCopyVersion(ctx)
	if err != nil {
		return err
	}
	tag := `"` + v + `"`
	_, _ = h.p.core.DB.ExecContext(ctx, `UPDATE pocket_phones SET copy_at = now() WHERE id = $1`, p.ID)
	c.Response().Header().Set("ETag", tag)
	c.Response().Header().Set("Cache-Control", "no-cache")
	if m := c.Request().Header.Get("If-None-Match"); m == tag || strings.Trim(m, `"`) == v {
		return c.NoContent(http.StatusNotModified)
	}
	cp, err := h.p.core.TillCopy(ctx)
	if err != nil {
		return err
	}
	return sendZipped(c, cp)
}

// sendZipped answers with JSON, gzipped for a client that takes it.
func sendZipped(c echo.Context, v any) error {
	if !strings.Contains(c.Request().Header.Get("Accept-Encoding"), "gzip") {
		return c.JSON(http.StatusOK, v)
	}
	c.Response().Header().Set("Content-Encoding", "gzip")
	c.Response().Header().Set("Content-Type", "application/json")
	c.Response().WriteHeader(http.StatusOK)
	zw := gzip.NewWriter(c.Response())
	if err := json.NewEncoder(zw).Encode(v); err != nil {
		return apperr.Internal("failed to send the copy", err)
	}
	return zw.Close()
}
