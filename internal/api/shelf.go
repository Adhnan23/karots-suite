package api

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
)

// Karots Shelf is the shop staff's product app (Android). It signs in as the
// person, never as their one device; its session token says so (claim app),
// and that session reaches only the routes below. The X-Ks-App header the app
// also sends grants nothing.
const (
	shelfHeader = "X-Ks-App"
	shelfApp    = "shelf"
)

// loginAppKey carries LoginRequest.App from bindLogin to Login.
const loginAppKey = "login_app"

// shelfNoAccess is what a person without Shelf access gets.
func shelfNoAccess() *apperr.AppError {
	return &apperr.AppError{Status: http.StatusForbidden, Code: "SHELF_NO_ACCESS", Message: "Ask the owner to give you Karots Shelf (Users page)"}
}

// fromShelf: the session was signed in for Karots Shelf (its token says so).
func fromShelf(c echo.Context) bool { return middleware.TokenApp(c) == shelfApp }

// shelfRequest: from Karots Shelf, by a person who may use it.
func shelfRequest(c echo.Context) bool {
	return fromShelf(c) && middleware.MayUseShelf(middleware.CurrentRole(c), middleware.CanUseShelf(c))
}

// shelfCore is every core admin route Karots Shelf calls, as Echo registers it.
// A route not here is refused to a cashier or sales person, even with the tick.
var shelfCore = []string{
	"GET /api/v1/admin/units",
	"POST /api/v1/admin/units",
	"GET /api/v1/admin/categories",
	"POST /api/v1/admin/categories/quick",
	"GET /api/v1/admin/supplier-options",
	"POST /api/v1/admin/suppliers",
	"GET /api/v1/admin/products/export",
	"GET /api/v1/admin/products/packs",
	"GET /api/v1/admin/products/:id",
	"PUT /api/v1/admin/products/:id",
	"DELETE /api/v1/admin/products/:id",
	"GET /api/v1/admin/products/:id/barcodes",
	"POST /api/v1/admin/products/:id/barcodes",
	"GET /api/v1/admin/products/:id/packs",
	"POST /api/v1/admin/products/:id/packs",
	"POST /api/v1/admin/inventory/intake/create",
	"POST /api/v1/admin/inventory/intake/restock",
	"POST /api/v1/admin/stock/adjust",
	"GET /api/v1/admin/stock/lots/:id",
	"POST /api/v1/admin/stock/batches/:id/expiry",
	"POST /api/v1/admin/stock/batches/:id/price",
}

// shelfAllowed: Karots Shelf may call this route (core or a plugin's).
func shelfAllowed(method, path string) bool {
	k := method + " " + path
	return slices.Contains(shelfCore, k) || slices.Contains(plugin.ShelfRoutes(), k)
}

// shelfGate holds a Karots Shelf session to Shelf's own routes, and stops it
// the moment the person's tick is taken away. Other sessions pass untouched.
func shelfGate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !fromShelf(c) {
			return next(c)
		}
		if !middleware.MayUseShelf(middleware.CurrentRole(c), middleware.CanUseShelf(c)) {
			return shelfNoAccess()
		}
		if !shelfAllowed(c.Request().Method, c.Path()) {
			return apperr.Forbidden("Karots Shelf can't do that")
		}
		return next(c)
	}
}

// adminOrShelf guards the admin routes: admin and manager as always; a cashier
// or sales person only from a Karots Shelf session (shelfGate has checked it).
func adminOrShelf(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if role := middleware.CurrentRole(c); role == auth.RoleAdmin || role == auth.RoleManager || fromShelf(c) {
			return next(c)
		}
		return apperr.Forbidden("")
	}
}
