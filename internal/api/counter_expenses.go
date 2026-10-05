package api

import (
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// CounterExpenses: category suggestions, taggable services, and the cashier's own
// expenses today (never the owner's — those are admin-only).
func (h *handlers) CounterExpenses(c echo.Context) error {
	ctx := c.Request().Context()
	svc := expenses.NewService(h.d.DB)
	distinct, err := expenses.NewRepository(h.d.DB).DistinctCategories(ctx)
	if err != nil {
		return err
	}
	svcs, err := h.t.products.ListServices(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	list, err := svc.List(ctx, expenses.Filter{From: &from})
	if err != nil {
		return err
	}
	out := ExpensesPage{Categories: expenses.MergedCategories(distinct), Services: []ServiceOption{}, Today: []CounterExpense{}}
	for _, p := range svcs {
		out.Services = append(out.Services, ServiceOption{ID: p.ID, Name: p.Name})
	}
	me := middleware.CurrentUserID(c)
	for _, e := range list {
		if e.PaidBy == nil || *e.PaidBy != me {
			continue
		}
		out.Today = append(out.Today, CounterExpense{ID: e.ID, Category: e.Category, Amount: dec2(e.Amount),
			Description: strOr(e.Description), ServiceName: strOr(e.ServiceName), PaidByName: strOr(e.PaidByName),
			CreatedAt: e.CreatedAt.Format(time.RFC3339)})
	}
	return c.JSON(200, out)
}

// CounterExpenseCreate records an expense paid from the drawer or a cashier
// locker: the expense row and its cash move commit together (karots-pos parity).
func (h *handlers) CounterExpenseCreate(c echo.Context) error {
	ctx := c.Request().Context()
	var req ExpenseRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	in := expenses.CreateInput{Category: strings.TrimSpace(req.Category), Amount: req.Amount, ServiceProductID: req.ServiceProductID}
	if d := strings.TrimSpace(req.Description); d != "" {
		in.Description = &d
	}
	if err := c.Validate(&in); err != nil { // no ExpenseDate: cashiers can't backdate
		return err
	}
	src, err := h.counterSource(c, req.LockerID)
	if err != nil {
		return err
	}
	id, rec, err := h.recordExpense(ctx, in, src, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "expense", strconv.FormatInt(id, 10), "recorded expense paid from "+rec.FromLabel+" at the counter")
	return h.afterCounterMove(ctx, c, src, req.CounterMoneyRequest, rec)
}
