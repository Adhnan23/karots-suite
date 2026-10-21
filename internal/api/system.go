package api

import (
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/palettes"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerSystem(v1 *echo.Group) {
	g := v1.Group("/system", append(h.protected, middleware.RequireSystemUser())...)
	g.GET("/appearance", h.GetAppearance)
	g.PUT("/appearance", h.PutAppearance)
	h.registerPalettes(g, v1)
}

func (h *handlers) GetAppearance(c echo.Context) error {
	st, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}

// PutAppearance stores the palette and receipt style. Unknown values fall back to
// defaults; a custom palette must exist. The legacy brand / radius can't be set any
// more: they stay while the palette is unchanged and clear when another is chosen.
func (h *handlers) PutAppearance(c echo.Context) error {
	var in Appearance
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	skin := settings.ValidSkin(in.Palette)
	if id, ok := palettes.IDOf(skin); ok {
		if _, err := h.palettes().Get(ctx, id); err != nil {
			return err
		}
	}
	cur, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	brand, radius := cur.SkinCustom, cur.SkinRadius
	if skin != cur.Skin {
		brand, radius = "", "rounded"
	}
	st, err := h.d.Settings.SetAppearance(ctx, skin, in.ReceiptStyle, brand, radius)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, appearanceOf(st))
}
