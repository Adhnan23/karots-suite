package api

import (
	"net/http"

	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// appearanceOf maps the forked settings columns to the API shape. Skin holds
// the chosen palette (studio/night) directly now; a brand override is
// independent of which palette is picked, so it always round-trips.
func appearanceOf(s *settings.Settings) Appearance {
	return Appearance{
		Palette:      settings.ValidSkin(s.Skin),
		BrandColor:   s.SkinCustom,
		Radius:       s.SkinRadius,
		Density:      s.Density,
		ReceiptStyle: s.ReceiptStyle,
	}
}

// Me is the SPA's boot call. JWT only — it must answer while locked or forced to
// change PIN, so the SPA can route to /lock or /account/pin.
func (h *handlers) Me(c echo.Context) error {
	ctx := c.Request().Context()
	me := Me{
		User: SessionUser{ID: middleware.CurrentUserID(c), Name: middleware.CurrentUserName(c), Role: middleware.CurrentRole(c)},
		Flags: Flags{
			CanHandleSuppliers: middleware.CanHandleSuppliers(c),
			CanManageCredit:    middleware.CanManageCredit(c),
			CanSeeCost:         middleware.CanSeeCost(c),
			CanExitKiosk:       middleware.CanExitKioskCtx(ctx),
			IsSystem:           middleware.IsSystemUserCtx(ctx),
		},
		Locked:        middleware.IsLocked(c),
		MustChangePin: middleware.MustChangePin(c),
		Plugins:       []string{},
	}
	for _, p := range plugin.All() {
		me.Plugins = append(me.Plugins, p.Name())
	}
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	me.ShopName, me.LockTimeoutMinutes, me.Appearance = st.ShopName, st.LockTimeoutMinutes, appearanceOf(st)
	return c.JSON(http.StatusOK, me)
}
