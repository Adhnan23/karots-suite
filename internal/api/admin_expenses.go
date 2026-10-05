package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminExpenses(g *echo.Group) {
	g.GET("/expenses", h.AdminExpenses)
	g.GET("/expenses/summary", h.AdminExpenseSummary)
	g.POST("/expenses", h.AdminExpenseCreate)
	g.PUT("/expenses/:id", h.AdminExpenseUpdate)
	g.GET("/expenses/:id/void", h.AdminExpenseVoidInfo)
	g.POST("/expenses/:id/void", h.AdminExpenseVoid)
}

// recordExpense books an expense and pays it from src in one tx — the expense
// row, the debit and its CR- receipt commit together or not at all. src
// External = paid from outside the shop: no money leg, no receipt. The admin
// form and the counter both come through here.
func (h *handlers) recordExpense(ctx context.Context, in expenses.CreateInput, src cashflow.Location, userID int64) (int64, *cashflow.Receipt, error) {
	reason := strings.TrimSpace(in.Category)
	if in.Description != nil && *in.Description != "" {
		reason += " - " + *in.Description
	}
	var id int64
	var rec *cashflow.Receipt
	err := db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		e, err := expenses.NewService(h.d.DB).CreateInTx(ctx, tx, in, userID)
		if err != nil || src.Kind == cashflow.KindExternal {
			if e != nil {
				id = e.ID
			}
			return err
		}
		id = e.ID
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: src, To: cashflow.External(), Amount: e.Amount,
			Reason: reason, ReceiptKind: "expense", Ref: &cashflow.Ref{Kind: "expense", ID: e.ID}, ActorID: userID})
		return err
	})
	return id, rec, err
}

// expenseFilter reads the list's range (default: unbounded), ?category= and ?q=.
func expenseFilter(c echo.Context) (expenses.Filter, error) {
	from, to, err := listRange(c)
	return expenses.Filter{From: from, To: to, Category: strings.TrimSpace(c.QueryParam("category")), Query: strings.TrimSpace(c.QueryParam("q"))}, err
}

// AdminExpenses pages the expenses (karots-pos /admin/expenses), newest first.
func (h *handlers) AdminExpenses(c echo.Context) error {
	f, err := expenseFilter(c)
	if err != nil {
		return err
	}
	f.Limit, f.Offset = counterPage, listOffset(c)
	rows, err := expenses.NewService(h.d.DB).List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	out := make([]AdminExpenseRow, 0, len(rows))
	for _, e := range rows {
		r := AdminExpenseRow{ID: e.ID, Date: e.ExpenseDate.Format("2006-01-02"), Category: e.Category, Amount: dec2(e.Amount),
			Description: strOr(e.Description), PaidBy: strOr(e.PaidByName), ServiceName: strOr(e.ServiceName), PaidFrom: strOr(e.PaidFrom)}
		if e.ServiceProductID != nil {
			r.ServiceID = *e.ServiceProductID
		}
		out = append(out, r)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminExpenseSummary is the list's head: the total of every match, the
// range's category chips, and the form's suggestions.
func (h *handlers) AdminExpenseSummary(c echo.Context) error {
	ctx := c.Request().Context()
	f, err := expenseFilter(c)
	if err != nil {
		return err
	}
	total, n, err := expenses.NewRepository(h.d.DB).Sum(ctx, f)
	if err != nil {
		return apperr.Internal("failed to total expenses", err)
	}
	from, to := time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), time.Now().AddDate(1, 0, 0)
	if f.From != nil {
		from, to = *f.From, *f.To
	}
	cats, err := reports.NewService(h.d.DB).ExpensesByCategory(ctx, from, to)
	if err != nil {
		return err
	}
	distinct, err := expenses.NewRepository(h.d.DB).DistinctCategories(ctx)
	if err != nil {
		return apperr.Internal("failed to load expense categories", err)
	}
	svcs, err := h.t.products.ListServices(ctx)
	if err != nil {
		return err
	}
	out := AdminExpenseSummary{Total: dec2(total), Count: n, ByCategory: []ExpenseCategoryTotal{}, Categories: expenses.MergedCategories(distinct), Services: []ServiceOption{}}
	for _, r := range cats {
		out.ByCategory = append(out.ByCategory, ExpenseCategoryTotal{Category: r.Category, Count: r.Count, Total: dec2(r.Total)})
	}
	for _, p := range svcs {
		out.Services = append(out.Services, ServiceOption{ID: p.ID, Name: p.Name})
	}
	return c.JSON(http.StatusOK, out)
}

func bindExpense(c echo.Context) (AdminExpenseRequest, expenses.CreateInput, error) {
	var req AdminExpenseRequest
	if err := c.Bind(&req); err != nil {
		return req, expenses.CreateInput{}, apperr.BadRequest("invalid expense")
	}
	amt, err := parseAmount(req.Amount, "amount")
	if err != nil {
		return req, expenses.CreateInput{}, err
	}
	in := expenses.CreateInput{Category: strings.TrimSpace(req.Category), Amount: amt.String(), ExpenseDate: strings.TrimSpace(req.Date), ServiceProductID: req.ServiceID}
	if d := strings.TrimSpace(req.Description); d != "" {
		in.Description = &d
	}
	return req, in, c.Validate(&in)
}

// AdminExpenseCreate records an expense paid from a locker, an open till, or
// outside the shop ("" — says so on the form; nothing moves).
func (h *handlers) AdminExpenseCreate(c echo.Context) error {
	req, in, err := bindExpense(c)
	if err != nil {
		return err
	}
	src, err := parseLocation(req.Source)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	id, rec, err := h.recordExpense(ctx, in, src, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	out := ReceiptRef{}
	detail := "recorded expense paid from outside the shop"
	if rec != nil {
		out.ReceiptID, detail = &rec.ID, "recorded expense paid from "+rec.FromLabel
	}
	h.audit(c, audit.ActionCreate, "expense", strconv.FormatInt(id, 10), detail)
	if src.Kind == cashflow.KindTill {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusCreated, out)
}

// AdminExpenseUpdate edits category, date, description and service. The amount
// changes only while no money moved for it (expenses.Service.Update says why);
// where it was paid from never changes — void and record it again.
func (h *handlers) AdminExpenseUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	_, in, err := bindExpense(c)
	if err != nil {
		return err
	}
	if err := expenses.NewService(h.d.DB).Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "expense", strconv.FormatInt(id, 10), "updated expense")
	return c.NoContent(http.StatusNoContent)
}

// AdminExpenseVoidInfo prepares the void sheet (karots-pos ExpenseVoidForm).
func (h *handlers) AdminExpenseVoidInfo(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if _, err := expenses.NewService(h.d.DB).Get(ctx, id); err != nil {
		return err
	}
	pay, err := expenses.PaidFrom(ctx, h.d.DB, id)
	if err != nil {
		return err
	}
	out := ExpenseVoidInfo{Paid: pay.Paid, From: pay.From, Owner: plugin.ExpenseClaim(ctx, id)}
	if pay.LockerID > 0 {
		if l, err := h.t.lockers.Get(ctx, pay.LockerID); err == nil && l.IsActive {
			out.Source = "locker:" + strconv.FormatInt(l.ID, 10)
		}
	}
	return c.JSON(http.StatusOK, out)
}

// AdminExpenseVoid voids an expense (karots-pos d66ac18): when money left a
// till or locker for it, the same amount goes back into the picked place in the
// same tx (CR- expense_void), then the row is deleted. Deleting used to leave
// the money out while the expense vanished from the books.
func (h *handlers) AdminExpenseVoid(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ExpenseVoidRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid void")
	}
	ctx := c.Request().Context()
	if owner := plugin.ExpenseClaim(ctx, id); owner != "" {
		return apperr.Validation("this expense belongs to " + owner + " — it can't be voided here")
	}
	dest, err := parseLocation(req.Dest)
	if err != nil {
		return err
	}
	var rec *cashflow.Receipt
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		rec, err = h.voidExpenseTx(ctx, tx, id, dest, middleware.CurrentUserID(c))
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "expense", strconv.FormatInt(id, 10), "voided expense")
	if rec == nil {
		return c.JSON(http.StatusCreated, ReceiptRef{})
	}
	if dest.Kind == cashflow.KindTill {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: &rec.ID})
}

// voidExpenseTx deletes an expense and, when a money move was booked for it,
// moves the same amount from outside back into dest — a place of ours, never
// an archived locker. Returns the CR- receipt, or nil when no money had moved.
func (h *handlers) voidExpenseTx(ctx context.Context, tx *sqlx.Tx, id int64, dest cashflow.Location, actorID int64) (*cashflow.Receipt, error) {
	pay, err := expenses.PaidFrom(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if pay.Paid {
		if dest.Kind == cashflow.KindExternal {
			return nil, apperr.Validation("this expense was paid from " + pay.From + " — pick where the money goes back")
		}
		if dest.Kind == cashflow.KindLocker {
			l, err := lockers.NewRepository(tx).Get(ctx, dest.ID)
			if err != nil || !l.IsActive {
				return nil, apperr.Validation("pick a locker that is still in use")
			}
		}
	}
	e, err := expenses.NewRepository(tx).DeleteReturning(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("expense")
	}
	if err != nil {
		return nil, apperr.Internal("failed to void expense", err)
	}
	if !pay.Paid {
		return nil, nil
	}
	reason := "voided expense: " + e.Category
	if e.Description != nil && *e.Description != "" {
		reason += " - " + *e.Description
	}
	return h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.External(), To: dest, Amount: e.Amount,
		Reason: reason, ReceiptKind: "expense_void", Ref: &cashflow.Ref{Kind: "expense", ID: id}, ActorID: actorID})
}
