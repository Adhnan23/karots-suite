package api

import (
	"slices"

	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// registerAdmin mounts the back-office API (karots-pos /admin): live, unlocked,
// PIN-chosen session, admin or manager only.
func (h *handlers) registerAdmin(v1 *echo.Group) {
	g := v1.Group("/admin", slices.Concat(h.protected, []echo.MiddlewareFunc{middleware.RequireRole(auth.RoleAdmin, auth.RoleManager)})...)
	h.registerAdminUnits(g)
	h.registerAdminCategories(g)
	h.registerAdminProducts(g)
	h.registerAdminServices(g)
	h.registerAdminGroups(g)
	h.registerAdminStock(g)
	h.registerAdminBatches(g)
	h.registerAdminStockTake(g)
	h.registerAdminIntake(g)
	h.registerAdminConversions(g)
	h.registerAdminMoney(g)
	h.registerAdminLockers(g)
	h.registerAdminLockerMoves(g)
	h.registerAdminExpenses(g)
	h.registerAdminCashflow(g)
	h.registerAdminTills(g)
	h.registerAdminSales(g)
	h.registerAdminFinance(g)
	h.registerAdminReports(g)
	h.registerAdminSuppliers(g)
	h.registerAdminCustomers(g)
	h.registerAdminPurchases(g)
	h.registerAdminReorder(g)
	h.registerAdminPurchaseReturns(g)
	h.registerAdminUsers(g)
	h.registerAdminData(g)
	h.registerAdminSettings(g)
	h.registerAdminImports(g)
	h.registerAdminSetup(g)
	g.POST("/labels/send", h.CounterLabels) // same sticker run as the counter (task 11 adds the admin page)
}
