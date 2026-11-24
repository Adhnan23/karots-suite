package catalogue

import (
	"context"
	"net/http"

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
