package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"
)

// Karots Shelf is the shop staff's product app (Android). It signs in as the
// person, never as their one device, and says so on every request.
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

// fromShelf: the request says it comes from Karots Shelf.
func fromShelf(c echo.Context) bool { return c.Request().Header.Get(shelfHeader) == shelfApp }

// shelfRequest: from Karots Shelf, by a person who may use it.
func shelfRequest(c echo.Context) bool {
	return fromShelf(c) && middleware.MayUseShelf(middleware.CurrentRole(c), middleware.CanUseShelf(c))
}
