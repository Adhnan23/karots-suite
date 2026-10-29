package api

import (
	"net/http"
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/palettes"

	"github.com/labstack/echo/v4"
)

// Custom palettes: CRUD for the system
// user, plus the public stylesheet of the active one (the login page needs it too).
func (h *handlers) registerPalettes(sys, v1 *echo.Group) {
	sys.GET("/palettes", h.ListPalettes)
	sys.GET("/palettes/:id", h.GetPalette)
	sys.POST("/palettes", h.CreatePalette)
	sys.PUT("/palettes/:id", h.UpdatePalette)
	sys.DELETE("/palettes/:id", h.DeletePalette)
	v1.GET("/appearance/palette.css", h.PaletteCSS)
}

func (h *handlers) palettes() *palettes.Service { return palettes.New(h.d.DB) }

func paletteOut(p palettes.Palette) Palette {
	return Palette{ID: p.ID, Theme: p.Theme, Name: p.Name, Base: p.Base, CSS: p.CSS, UpdatedAt: p.UpdatedAt}
}

func paletteID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, apperr.NotFound("palette")
	}
	return id, nil
}

func (h *handlers) ListPalettes(c echo.Context) error {
	ps, err := h.palettes().List(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]Palette, len(ps))
	for i, p := range ps {
		out[i] = paletteOut(p)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) GetPalette(c echo.Context) error {
	id, err := paletteID(c)
	if err != nil {
		return err
	}
	p, err := h.palettes().Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, paletteOut(p))
}

func (h *handlers) CreatePalette(c echo.Context) error {
	var in PaletteInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	p, err := h.palettes().Create(c.Request().Context(), palettes.Input{Name: in.Name, Base: in.Base, CSS: in.CSS})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, paletteOut(p))
}

func (h *handlers) UpdatePalette(c echo.Context) error {
	id, err := paletteID(c)
	if err != nil {
		return err
	}
	var in PaletteInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	p, err := h.palettes().Update(c.Request().Context(), id, palettes.Input{Name: in.Name, Base: in.Base, CSS: in.CSS})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, paletteOut(p))
}

func (h *handlers) DeletePalette(c echo.Context) error {
	id, err := paletteID(c)
	if err != nil {
		return err
	}
	st, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return err
	}
	if err := h.palettes().Delete(c.Request().Context(), id, st.Skin); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// PaletteCSS is public: the login page links it before anyone signs in. Empty for a
// built-in palette (those are in the bundle). Served as text/css, never inlined.
func (h *handlers) PaletteCSS(c echo.Context) error {
	ctx := c.Request().Context()
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	css, err := h.palettes().ActiveCSS(ctx, st.Skin)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.Blob(http.StatusOK, "text/css; charset=utf-8", []byte(css))
}
