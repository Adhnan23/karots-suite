package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func decOpt(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return dec2(*d)
}

// CounterZReport is the day-end (Z) read for one drawer session: sales, tender
// split, cash reconciliation and movements. Cashiers see only their own
// sessions; admin/manager any (karots-pos cashier ZReport).
func (h *handlers) CounterZReport(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	sess, moves, err := h.d.CashRegister.SessionDetail(ctx, id)
	if err != nil {
		return err
	}
	role := middleware.CurrentRole(c)
	if role != auth.RoleAdmin && role != auth.RoleManager && sess.UserID != middleware.CurrentUserID(c) {
		return apperr.Forbidden("you can only see your own shift")
	}
	to := time.Now()
	out := ZReport{SessionID: sess.ID, CashierName: sess.UserName, OpenedAt: sess.OpenedAt.Format(time.RFC3339),
		OpeningCash: dec2(sess.OpeningCash), ExpectedCash: decOpt(sess.ExpectedCash), ClosingCash: decOpt(sess.ClosingCash),
		Difference: decOpt(sess.Difference), ByMethod: []ZMethod{}, Moves: []ZMove{}, Floats: []ZFloatGroup{}}
	if sess.ClosedAt != nil {
		to = *sess.ClosedAt
		out.ClosedAt = to.Format(time.RFC3339)
	}
	sum, err := h.d.Sales.PeriodSummary(ctx, sess.UserID, sess.OpenedAt, to)
	if err != nil {
		return err
	}
	out.SalesCount, out.Gross, out.Discount, out.Returns, out.Net = sum.Count, dec2(sum.Gross), dec2(sum.Discount), dec2(sum.Returns), dec2(sum.Net)
	for _, m := range sum.ByMethod {
		out.ByMethod = append(out.ByMethod, ZMethod{Method: m.Method, Amount: dec2(m.Amount)})
	}
	for _, m := range moves {
		out.Moves = append(out.Moves, ZMove{At: m.CreatedAt.Format(time.RFC3339), Type: m.Type, Reason: strOr(m.Reason), Amount: dec2(m.Amount)})
	}
	for _, g := range plugin.CollectMoneyLocations(ctx) {
		fg := ZFloatGroup{Group: g.Group, Total: dec2(g.Total), Items: []ZFloatItem{}}
		for _, it := range g.Items {
			fg.Items = append(fg.Items, ZFloatItem{Name: it.Name, Balance: dec2(it.Balance)})
		}
		out.Floats = append(out.Floats, fg)
	}
	return c.JSON(http.StatusOK, out)
}
