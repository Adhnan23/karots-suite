package pocketcashier

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// parts hands a phone what the compiled-in plugins' screens need offline. A part
// that fails fails the call, so the phone keeps its last good parts.
func (h handlers) parts(c echo.Context) error {
	ctx := c.Request().Context()
	all := map[string]any{}
	for _, d := range plugin.OfflineDataAll() {
		v, err := d.Build(ctx)
		if err != nil {
			return apperr.Internal("failed to read "+d.Key+" for the phone", err)
		}
		all[d.Key] = v
	}
	raw, err := json.Marshal(all) // map keys sort, so the same data hashes the same
	if err != nil {
		return apperr.Internal("failed to encode the parts", err)
	}
	sum := md5.Sum(raw)
	tag := `"` + hex.EncodeToString(sum[:8]) + `"`
	c.Response().Header().Set("ETag", tag)
	c.Response().Header().Set("Cache-Control", "no-cache")
	if m := c.Request().Header.Get("If-None-Match"); m == tag || strings.Trim(m, `"`) == strings.Trim(tag, `"`) {
		return c.NoContent(http.StatusNotModified)
	}
	return sendZipped(c, json.RawMessage(raw))
}
