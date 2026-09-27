package api

import (
	"net/http"

	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// appearanceOf maps the forked settings columns to the API shape. The new
// product has one theme, so the old "skin" only signals whether a brand colour
// is set ("custom").
func appearanceOf(s *settings.Settings) Appearance {
	a := Appearance{Radius: s.SkinRadius, Density: s.Density, ReceiptStyle: s.ReceiptStyle}
	if s.Skin == "custom" {
		a.BrandColor = s.SkinCustom
	}
	return a
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
