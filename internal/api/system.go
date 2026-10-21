package api

import (
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerSystem(v1 *echo.Group) {
	g := v1.Group("/system", append(h.protected, middleware.RequireSystemUser())...)
	g.GET("/appearance", h.GetAppearance)
	g.PUT("/appearance", h.PutAppearance)
}

func (h *handlers) GetAppearance(c echo.Context) error {
	st, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}

// PutAppearance stores the locked look. Unknown values fall back to defaults in
// the forked settings validators. The palette and the brand override are
// independent: an unknown palette falls back to "studio", but the brand colour
// (if valid) is kept regardless of which palette is chosen.
func (h *handlers) PutAppearance(c echo.Context) error {
	var in Appearance
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	st, err := h.d.Settings.SetAppearance(c.Request().Context(), settings.ValidSkin(in.Palette), in.ReceiptStyle, in.BrandColor, in.Radius)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}
