package api

import (
	"context"
	"net/http"

	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminSetup(g *echo.Group) {
	g.GET("/setup", h.AdminSetup, middleware.RequireRole(auth.RoleAdmin))
}

// setupSteps computes the go-live checklist from live data (karots-pos
// setupStatus). Best-effort: a failed count leaves its step not done.
func setupSteps(ctx context.Context, q sqlx.QueryerContext) []SetupStep {
	yes := func(query string) bool {
		var ok bool
		_ = sqlx.GetContext(ctx, q, &ok, query)
		return ok
	}
	return []SetupStep{
		// "My Shop" is the migration default, not a real name.
		{"name", "Name your shop", "Name, address and currency for the receipt", "/admin/settings",
			yes(`SELECT btrim(shop_name) <> '' AND lower(btrim(shop_name)) <> 'my shop' FROM settings WHERE id = 1`)},
		{"staff", "Add a staff login", "At least one cashier or manager", "/admin/users",
			yes(`SELECT EXISTS (SELECT 1 FROM users WHERE is_active AND NOT is_system)`)},
		{"categories", "Create categories", "Group your products", "/admin/categories",
			yes(`SELECT EXISTS (SELECT 1 FROM categories)`)},
		{"products", "Add products", "Type them in, or import a sheet", "/admin/products",
			yes(`SELECT EXISTS (SELECT 1 FROM products WHERE is_active)`)},
		// A shop trades without a locker, but the day's takings then go nowhere
		// recorded at close; the default locker makes the till pre-pick it.
		{"locker", "Set up a cash safe", "Add a locker, then make it the default in Settings", "/admin/lockers",
			yes(`SELECT default_locker_id IS NOT NULL FROM settings WHERE id = 1`)},
		{"printer", "Set up the receipt printer", "Pick it in Settings and print a test slip", "/admin/settings",
			yes(`SELECT btrim(receipt_printer) <> '' FROM settings WHERE id = 1`)},
		{"sale", "Make your first sale", "Ring one up at the till", "/till",
			yes(`SELECT EXISTS (SELECT 1 FROM sales)`)},
	}
}

func (h *handlers) AdminSetup(c echo.Context) error {
	return c.JSON(http.StatusOK, setupSteps(c.Request().Context(), h.d.DB))
}
