package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"
)

// salesPersonWrites is everything a sales person may change at the till. It is a
// list on purpose: a new till route is refused for them until it's added here.
// Plugins' routes are outside it; a plugin route that moves money carries
// middleware.RequireHandlesMoney.
var salesPersonWrites = []string{
	"POST /api/v1/till/held",
	"DELETE /api/v1/till/held/:id",
	"POST /api/v1/till/tickets/:id/open",
	"POST /api/v1/till/customers",
	"POST /api/v1/till/quick-item",
	"POST /api/v1/till/products/:id/barcode",
	"POST /api/v1/till/products/:id/barcodes",
	"POST /api/v1/till/products/:id/packs",
	"POST /api/v1/till/labels",
	"POST /api/v1/till/labels/preview",
	"POST /api/v1/till/labels/text",
	"POST /api/v1/till/labels/text/preview",
}

// noMoney keeps a sales person's till to reading and preparing carts.
func noMoney(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !middleware.SendsToCashier(c) || c.Request().Method == http.MethodGet ||
			!strings.HasPrefix(c.Path(), "/api/v1/till/") && !strings.HasPrefix(c.Path(), "/api/v1/cheques") ||
			slices.Contains(salesPersonWrites, c.Request().Method+" "+c.Path()) {
			return next(c)
		}
		return apperr.Forbidden("a sales person doesn't handle money — send it to the cashier")
	}
}
