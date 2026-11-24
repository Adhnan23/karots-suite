package catalogue

import (
	"context"
	"net/http"
	"strconv"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

type handlers struct{ p *Plugin }

// Status is the copy's state for the plugin's page and the pickers (CTL-4: no copy yet = type by hand).
type Status struct {
	State
	HasAddress bool `json:"hasAddress"`
}

func (h handlers) status(c echo.Context) error {
	st, err := h.p.store.State(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, Status{State: st, HasAddress: address() != ""})
}

// check is the page's Check now: a sync right away; a failure shows on the status (the copy stays).
func (h handlers) check(c echo.Context) error {
	_, _ = h.p.sync.Sync(context.WithoutCancel(c.Request().Context())) // leaving the page doesn't stop it
	return h.status(c)
}

// search is the pickers' search on this PC's copy (CTL-5); cashier PCs, phones and Shelf ask the main PC.
func (h handlers) search(c echo.Context) error {
	q := c.QueryParam("q")
	if len(q) > 200 {
		return apperr.BadRequest("search is too long")
	}
	out, err := h.p.store.Search(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func productParam(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("product"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("which product?")
	}
	return id, nil
}

// waiting is a product's waiting catalogue change (CTL-8), for its Items to review buttons.
func (h handlers) waiting(c echo.Context) error {
	id, err := productParam(c)
	if err != nil {
		return err
	}
	cs, err := h.p.store.Waiting(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}

// use is "Use the catalogue's" (CTL-9): the name through core's rename, fields through their plugin.
func (h handlers) use(c echo.Context) error {
	id, err := productParam(c)
	if err != nil {
		return err
	}
	if err := h.p.store.useCatalogue(c.Request().Context(), id, h.p.core.Products.Rename); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// keep is "Keep mine" (CTL-9).
func (h handlers) keep(c echo.Context) error {
	id, err := productParam(c)
	if err != nil {
		return err
	}
	if err := h.p.store.keepMine(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
