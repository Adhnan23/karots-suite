package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/units"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminUnits(g *echo.Group) {
	g.GET("/units", h.TillUnits) // same list the till reads
	g.POST("/units", h.AdminUnitCreate)
	g.PUT("/units/:id", h.AdminUnitUpdate)
	g.DELETE("/units/:id", h.AdminUnitDelete)
}

func bindUnit(c echo.Context) (units.Input, error) {
	var req UnitRequest
	if err := c.Bind(&req); err != nil {
		return units.Input{}, apperr.BadRequest("invalid request body")
	}
	in := units.Input{Name: strings.TrimSpace(req.Name), Abbreviation: strings.TrimSpace(req.Abbreviation), AllowDecimal: req.AllowDecimal}
	return in, c.Validate(&in)
}

func (h *handlers) AdminUnitCreate(c echo.Context) error {
	in, err := bindUnit(c)
	if err != nil {
		return err
	}
	u, err := h.t.units.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "unit", strconv.FormatInt(u.ID, 10), "created unit "+u.Name)
	return c.JSON(http.StatusCreated, Unit{ID: u.ID, Name: u.Name, Abbreviation: u.Abbreviation, AllowDecimal: u.AllowDecimal})
}

func (h *handlers) AdminUnitUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, err := bindUnit(c)
	if err != nil {
		return err
	}
	if err := h.t.units.Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "unit", strconv.FormatInt(id, 10), "updated unit "+in.Name)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminUnitDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.units.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "unit", strconv.FormatInt(id, 10), "deleted unit")
	return c.NoContent(http.StatusNoContent)
}
