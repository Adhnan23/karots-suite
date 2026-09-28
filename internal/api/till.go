package api

import (
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/denominations"
	"karots-suite/internal/features/heldsales"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/units"
	"karots-suite/internal/features/warranty"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// tillServices are the seam-free domain services the till calls, built once from
// the DB. Services that carry core seams (sales, cash register, drawer kick) come
// in through Deps instead, wired in app.NewServices.
type tillServices struct {
	products  *products.Service
	groups    *productgroups.Service
	units     *units.Service
	audit     *audit.Service
	customers *customers.Service
	stock     *stock.Service
	denoms    *denominations.Service
	lockers   *lockers.Service
	receipts  *cashflow.ReceiptService
	held      *heldsales.Service
	warranty  *warranty.Service
}

func newTillServices(db *sqlx.DB) tillServices {
	return tillServices{
		products:  products.NewService(db),
		groups:    productgroups.NewService(db),
		units:     units.NewService(db),
		audit:     audit.NewService(db),
		customers: customers.NewService(db),
		stock:     stock.NewService(db),
		denoms:    denominations.NewService(db),
		lockers:   lockers.NewService(db),
		receipts:  cashflow.NewReceiptService(db),
		held:      heldsales.NewService(db),
		warranty:  warranty.NewService(db),
	}
}

// registerTill mounts the cashier API. Every route needs a live, unlocked,
// PIN-chosen session (h.protected); per-route guards add role/flag checks.
func (h *handlers) registerTill(v1 *echo.Group) {
	g := v1.Group("/till", h.protected...)
	h.registerTillCatalog(g)
	h.registerTillCustomers(g)
	h.registerTillDrawer(g)
	h.registerTillSales(g)
}

func dec2(d decimal.Decimal) string { return d.StringFixed(2) }

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func pathID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid id")
	}
	return id, nil
}

// audit records a till action for the owner's trail. The hidden system account
// is never audited (spec §6.6).
func (h *handlers) audit(c echo.Context, action, entity, id, detail string) {
	ctx := c.Request().Context()
	if middleware.IsSystemUserCtx(ctx) {
		return
	}
	h.t.audit.Record(ctx, middleware.CurrentUserID(c), action, entity, id, detail)
}
