package api

import (
	"context"
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// registerCounter mounts the till's counter tabs (M1b). g is /till, already protected.
func (h *handlers) registerCounter(g *echo.Group) {
	g.GET("/sales", h.CounterSales)
	g.GET("/sales/:id", h.CounterSale)
	g.POST("/sales/:id/return", h.CounterReturn)
	g.POST("/sales/:id/refund-print", h.CounterPrintRefund)
	g.POST("/customers/:id/payment", h.CounterCreditPayment)
	g.POST("/customers/:id/refund", h.CounterAdvanceRefund)
	g.POST("/customer-payments/:id/print", h.CounterPrintDebtSlip)
	g.GET("/z/:id", h.CounterZReport)
	g.GET("/money-receipts", h.CounterMoneyReceipts)
	g.GET("/customer-payments", h.CounterDebtReceipts)
	g.GET("/warranty", h.CounterWarrantyUnits)
	g.GET("/warranty/serial/:serial", h.CounterWarrantySerial)
	g.GET("/warranty/receipt", h.CounterWarrantyReceipt)
	g.POST("/warranty/replace", h.CounterWarrantyReplace)
	g.POST("/warranty/receipt-replace", h.CounterWarrantyReceiptReplace)
	g.GET("/warranty-claims", h.CounterWarrantyClaims)
	g.POST("/warranty-claims/:id/print", h.CounterPrintWarrantySlip)
	g.POST("/labels", h.CounterLabels)
	g.GET("/conversions", h.CounterConversions)
	g.POST("/conversions/:id/run", h.CounterConversionRun)
	sup := g.Group("", middleware.RequireSupplierAccess())
	sup.GET("/expenses", h.CounterExpenses)
	sup.POST("/expenses", h.CounterExpenseCreate)
	sup.GET("/suppliers", h.CounterSuppliers)
	sup.GET("/suppliers/:id", h.CounterSupplier)
	sup.POST("/suppliers/:id/pay", h.CounterSupplierPay)
	sup.POST("/suppliers/:id/refund", h.CounterSupplierRefund)
	sup.POST("/suppliers", h.CounterSupplierCreate)
	sup.POST("/suppliers/:id/receive", h.CounterReceiveWalkIn)
	sup.POST("/suppliers/:id/order", h.CounterOrderCreate)
	sup.GET("/orders/:id", h.CounterOrder)
	sup.POST("/orders/:id/receive", h.CounterReceiveOrder)
	sup.POST("/intake-products", h.CounterIntakeProduct)
}

// counterSource resolves where counter cash comes from: the cashier's own drawer
// (0) or a locker the owner opened to cashiers. Anything else is refused.
func (h *handlers) counterSource(c echo.Context, lockerID int64) (cashflow.Location, error) {
	if lockerID == 0 {
		return cashflow.Till(middleware.CurrentUserID(c)), nil
	}
	ls, err := h.t.lockers.ListForCashier(c.Request().Context())
	if err != nil {
		return cashflow.Location{}, err
	}
	for _, l := range ls {
		if l.ID == lockerID {
			return cashflow.Locker(l.ID), nil
		}
	}
	return cashflow.Location{}, apperr.Forbidden("you can't take cash from there")
}

// counterSlip is afterCounterMove for moves that may not touch the counter at
// all (card/online with no account): then there's no slip and nothing to pop.
func (h *handlers) counterSlip(c echo.Context, loc cashflow.Location, req CounterMoneyRequest, rec *cashflow.Receipt) error {
	if rec == nil {
		return c.JSON(http.StatusCreated, ReceiptRef{})
	}
	return h.afterCounterMove(c.Request().Context(), c, loc, req, rec)
}

// afterCounterMove pops the drawer for a till cash move unless the SPA will
// auto-print the slip with the pulse merged in, and answers with the slip ref.
func (h *handlers) afterCounterMove(ctx context.Context, c echo.Context, src cashflow.Location, req CounterMoneyRequest, rec *cashflow.Receipt) error {
	if src.Kind == cashflow.KindTill && !req.DeferDrawerKick {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: &rec.ID})
}
