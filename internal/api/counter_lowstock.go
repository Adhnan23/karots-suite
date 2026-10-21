package api

import (
	"net/http"

	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// CounterLowStock is the till's Low stock tab: the same worklist as Reorder
// (filters, paging, reason numbers), for a cashier the owner trusts with it.
// Cost only with See cost; ordering goes through the Suppliers tab.
func (h *handlers) CounterLowStock(c echo.Context) error {
	out, err := h.buildReorderList(c, middleware.MaySeeCost(middleware.CurrentRole(c), middleware.CanSeeCost(c)))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}
