package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/settings"

	"github.com/labstack/echo/v4"
)

func sellLayoutsOf(l settings.SellLayouts) map[string]SellLayout {
	out := map[string]SellLayout{}
	for k, v := range l {
		out[k] = SellLayout(v)
	}
	return out
}

func (h *handlers) sellLayoutPage(c echo.Context, l settings.SellLayouts) error {
	sizes, err := h.screens.List(c.Request().Context(), time.Now())
	if err != nil {
		return err
	}
	page := SellLayoutPage{Layouts: sellLayoutsOf(l), Screens: []ScreenSize{}}
	for _, s := range sizes {
		page.Screens = append(page.Screens, ScreenSize{W: s.W, H: s.H, Users: []string(s.Users), LastSeen: s.LastSeen.UTC().Format(time.RFC3339)})
	}
	return c.JSON(http.StatusOK, page)
}

func (h *handlers) GetSellLayout(c echo.Context) error {
	st, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return err
	}
	return h.sellLayoutPage(c, st.SellLayouts)
}

// PutSellLayout replaces every saved layout; a bad one refuses the whole save.
func (h *handlers) PutSellLayout(c echo.Context) error {
	var in SellLayoutSave
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	m := map[string]settings.SellLayout{}
	for k, v := range in.Layouts {
		m[k] = settings.SellLayout(v)
	}
	l, err := h.d.Settings.SetSellLayouts(c.Request().Context(), m)
	if err != nil {
		return err
	}
	return h.sellLayoutPage(c, l)
}
