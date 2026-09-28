package api

import (
	"encoding/json"
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerTillDrawer(g *echo.Group) {
	g.GET("/drawer", h.TillDrawer)
	g.POST("/drawer/open", h.TillOpenDrawer)
	g.POST("/drawer/close", h.TillCloseDrawer)
	g.POST("/drawer/withdraw", h.TillWithdraw)
	g.POST("/drawer/deposit", h.TillDeposit)
	g.POST("/drawer/no-sale", h.TillNoSale)
	g.POST("/money-receipts/:id/print", h.TillPrintMoneyReceipt)
}

func drawerSummary(sum *cashregister.Summary) DrawerSummary {
	out := DrawerSummary{CashSales: dec2(sum.CashSales), PayIns: dec2(sum.PayIns),
		Withdrawals: dec2(sum.Withdrawals), Expected: dec2(sum.Expected)}
	if s := sum.Session; s != nil {
		out.Session = &DrawerSession{ID: s.ID, OpeningCash: dec2(s.OpeningCash), OpenedAt: s.OpenedAt.Format(time.RFC3339)}
	}
	if sum.LastClosing != nil {
		v := dec2(*sum.LastClosing)
		out.LastClosing = &v
	}
	if len(sum.LastBreakdown) > 0 {
		var m map[string]float64
		if json.Unmarshal(sum.LastBreakdown, &m) == nil {
			out.LastBreakdown = m
		}
	}
	return out
}

// TillDrawer is one call for everything the drawer UI needs: this user's session
// summary, the active denominations, and the cashier-accessible lockers.
func (h *handlers) TillDrawer(c echo.Context) error {
	ctx := c.Request().Context()
	sum, err := h.d.CashRegister.Summary(ctx, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	dens, err := h.t.denoms.List(ctx, true)
	if err != nil {
		return err
	}
	lks, err := h.t.lockers.ListForCashier(ctx)
	if err != nil {
		return err
	}
	out := DrawerState{Summary: drawerSummary(sum), Denominations: make([]Denomination, 0, len(dens)), Lockers: make([]Locker, 0, len(lks))}
	for _, d := range dens {
		out.Denominations = append(out.Denominations, Denomination{Value: d.Value.String(), IsNote: d.IsNote})
	}
	for _, l := range lks {
		out.Lockers = append(out.Lockers, Locker{ID: l.ID, Name: l.Name, Kind: l.Kind, Balance: dec2(l.Balance)})
	}
	return c.JSON(http.StatusOK, out)
}

func breakdownJSON(m map[string]float64) json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func (h *handlers) TillOpenDrawer(c echo.Context) error {
	var req OpenDrawerRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	sess, err := h.d.CashRegister.Open(c.Request().Context(), middleware.CurrentUserID(c),
		cashregister.OpenInput{Breakdown: breakdownJSON(req.Breakdown), SourceLockerID: req.SourceLockerID})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: sess.ReceiptID})
}

func (h *handlers) TillCloseDrawer(c echo.Context) error {
	var req CloseDrawerRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	res, err := h.d.CashRegister.Close(c.Request().Context(), middleware.CurrentUserID(c),
		cashregister.CloseInput{Breakdown: breakdownJSON(req.Breakdown), DestLockerID: req.DestLockerID})
	if err != nil {
		return err
	}
	out := CloseResult{SessionID: res.Session.ID, CashSales: dec2(res.CashSales), ExpectedCash: dec2(res.ExpectedCash),
		Difference: dec2(res.Difference), Counted: "0.00", ReceiptID: res.ReceiptID}
	if res.Session.ClosingCash != nil {
		out.Counted = dec2(*res.Session.ClosingCash)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) moneyMove(c echo.Context, move func(ctx echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error)) error {
	var req MoneyMoveRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := cashregister.MovementInput{Amount: req.Amount, Reason: req.Reason, CounterLockerID: req.LockerID, DeferDrawerKick: req.DeferDrawerKick}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sum, err := move(c, in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, ReceiptRef{ReceiptID: sum.ReceiptID})
}

func (h *handlers) TillWithdraw(c echo.Context) error {
	return h.moneyMove(c, func(c echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error) {
		return h.d.CashRegister.Withdraw(c.Request().Context(), middleware.CurrentUserID(c), in)
	})
}

func (h *handlers) TillDeposit(c echo.Context) error {
	return h.moneyMove(c, func(c echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error) {
		return h.d.CashRegister.PayIn(c.Request().Context(), middleware.CurrentUserID(c), in)
	})
}

// TillNoSale pops the drawer with no transaction. Only when the shop enabled a
// drawer, and always audited — a no-sale open is a classic theft surface.
func (h *handlers) TillNoSale(c echo.Context) error {
	ctx := c.Request().Context()
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if st == nil || !st.OpenCashDrawer {
		return apperr.Forbidden("cash drawer is not enabled")
	}
	h.d.KickDrawer(ctx)
	h.audit(c, audit.ActionUpdate, "cash_drawer", "", "opened drawer (no sale)")
	return c.NoContent(http.StatusNoContent)
}
