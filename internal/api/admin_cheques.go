package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// registerAdminCheques mounts Money → Cheques: only admin and manager review
// or correct a cheque.
func (h *handlers) registerAdminCheques(g *echo.Group) {
	g.GET("/cheques", h.AdminCheques)
	g.GET("/cheques/summary", h.AdminChequeSummary)
	g.GET("/cheques/:id", h.AdminCheque)
	g.POST("/cheques/:id/clear", h.AdminChequeClear)
	g.POST("/cheques/:id/return", h.AdminChequeReturn)
	g.PUT("/cheques/:id", h.AdminChequeFix)
}

// registerCheques mounts what any signed-in user's cheque form asks: bank
// name suggestions and the repeated-cheque warning.
func (h *handlers) registerCheques(g *echo.Group) {
	g.GET("/banks", h.ChequeBanks)
	g.GET("/duplicate", h.ChequeDuplicate)
}

func (h *handlers) cheques() *cheques.Service { return cheques.NewService(h.d.DB, h.d.Cashflow) }

func localDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func chequeRow(c cheques.Cheque, today time.Time) ChequeRow {
	r := ChequeRow{ID: c.ID, Direction: c.Direction, ChequeDate: c.ChequeDate.Format("2006-01-02"), Bank: c.Bank,
		LockerName: strOr(c.LockerName), Number: c.Number, Amount: dec2(c.Amount), Status: c.Status,
		Days: int(localDay(c.ChequeDate).Sub(today).Hours() / 24)}
	if c.LockerID != nil {
		r.LockerID = *c.LockerID
	}
	if c.StatusDate != nil {
		r.StatusDate = c.StatusDate.Format("2006-01-02")
	}
	switch {
	case c.CustomerName != nil:
		r.Who = *c.CustomerName
	case c.SupplierName != nil:
		r.Who = *c.SupplierName
	case c.ExpenseCategory != nil:
		r.Who = *c.ExpenseCategory
	}
	return r
}

func qInt(c echo.Context, name string) int64 {
	v, _ := strconv.ParseInt(c.QueryParam(name), 10, 64)
	return v
}

// AdminCheques pages the cheques, Pending first, by cheque date.
func (h *handlers) AdminCheques(c echo.Context) error {
	f := cheques.Filter{Direction: c.QueryParam("dir"), Status: c.QueryParam("status"), CustomerID: qInt(c, "customer"),
		SupplierID: qInt(c, "supplier"), LockerID: qInt(c, "locker"), Query: c.QueryParam("q"), Limit: counterPage, Offset: listOffset(c)}
	// The cheque date in the list's range (presets or from/to; the end is exclusive).
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	if from != nil {
		last := to.AddDate(0, 0, -1)
		f.From, f.To = from, &last
	}
	today := localDay(time.Now())
	if c.QueryParam("due") == "1" { // the dashboard reminders' lists
		end := today
		if f.Direction == "out" {
			end = today.AddDate(0, 0, 3)
		}
		f.To = &end
	}
	rows, err := h.cheques().List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	out := make([]ChequeRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, chequeRow(r, today))
	}
	return c.JSON(http.StatusOK, out)
}

func chequeSummary(sm cheques.Summary) ChequeSummary {
	out := ChequeSummary{InHand: dec2(sm.InHand), Given: dec2(sm.Given), DueWeek: dec2(sm.DueWeek),
		ToDeposit: CountTotal{Count: sm.ToDeposit.Count, Total: dec2(sm.ToDeposit.Total)},
		GivenDue:  CountTotal{Count: sm.GivenDue.Count, Total: dec2(sm.GivenDue.Total)}, Short: []BankAfter{}}
	for _, b := range sm.Short {
		out.Short = append(out.Short, BankAfter{LockerID: b.LockerID, Name: b.Name, After: dec2(b.After)})
	}
	return out
}

func (h *handlers) AdminChequeSummary(c echo.Context) error {
	sm, err := h.cheques().Summary(c.Request().Context(), time.Now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, chequeSummary(sm))
}

// AdminCheque is one cheque: where it came from, its history and (for a
// received one) the bank place the last cheque cleared into.
func (h *handlers) AdminCheque(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	ch, err := h.cheques().Get(ctx, id)
	if err != nil {
		return err
	}
	out := ChequeDetail{ChequeRow: chequeRow(*ch, localDay(time.Now())), Note: ch.Note, History: []ChequeEvent{}}
	if ch.CustomerID != nil {
		out.CustomerID = *ch.CustomerID
	}
	if ch.SupplierID != nil {
		out.SupplierID = *ch.SupplierID
	}
	q := h.d.DB
	switch {
	case ch.SaleID != nil:
		out.Source = ChequeSource{Kind: "sale", ID: *ch.SaleID}
		q.GetContext(ctx, &out.Source.Label, `SELECT 'Sale ' || receipt_no FROM sales WHERE id = $1`, *ch.SaleID) //nolint:errcheck // label only
	case ch.CustomerPaymentID != nil:
		out.Source = ChequeSource{Kind: "customer_payment", ID: *ch.CustomerPaymentID}
		q.GetContext(ctx, &out.Source.Label, `SELECT 'Credit payment ' || COALESCE(receipt_no, '') FROM customer_payments WHERE id = $1`, *ch.CustomerPaymentID) //nolint:errcheck
	case ch.SupplierPaymentID != nil:
		out.Source = ChequeSource{Kind: "supplier_payment", ID: *ch.SupplierPaymentID, Label: "Payment to " + strOr(ch.SupplierName)}
	case ch.ExpenseID != nil:
		out.Source = ChequeSource{Kind: "expense", ID: *ch.ExpenseID, Label: "Expense: " + strOr(ch.ExpenseCategory)}
	}
	entries, err := audit.NewRepository(q).List(ctx, audit.ListFilter{Entity: "cheque", EntityID: strconv.FormatInt(id, 10), Limit: 50})
	if err != nil {
		return apperr.Internal("failed to load the cheque's history", err)
	}
	for _, e := range entries {
		out.History = append(out.History, ChequeEvent{At: e.CreatedAt.Format(time.RFC3339), User: e.UserName, Detail: strOr(e.Detail)})
	}
	q.GetContext(ctx, &out.LastBankID, `SELECT COALESCE((SELECT locker_id FROM cheques WHERE direction = 'in' AND status = 'cleared'
		ORDER BY status_date DESC, id DESC LIMIT 1), 0)`) //nolint:errcheck // a pre-fill only
	return c.JSON(http.StatusOK, out)
}

// AdminChequeClear marks a cheque Cleared (received: into the picked bank
// place; given: the money leaves the bank it was drawn on).
func (h *handlers) AdminChequeClear(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ChequeClearRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	on := time.Now()
	if d := strings.TrimSpace(req.Date); d != "" {
		if on, err = time.ParseInLocation("2006-01-02", d, time.Local); err != nil {
			return apperr.Validation("date must be YYYY-MM-DD")
		}
	}
	if err := h.cheques().Clear(c.Request().Context(), id, on, req.LockerID, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminChequeReturn marks a cheque Bounced or Cancelled: the amount is owed again.
func (h *handlers) AdminChequeReturn(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ChequeReturnRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.cheques().Return(c.Request().Context(), id, req.Status, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminChequeFix corrects a pending cheque's bank / number / date / note.
func (h *handlers) AdminChequeFix(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ChequeFixRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := cheques.FixInput{Bank: strings.TrimSpace(req.Bank), LockerID: req.LockerID, Number: strings.TrimSpace(req.Number),
		Date: strings.TrimSpace(req.Date), Note: strings.TrimSpace(req.Note)}
	if err := h.cheques().Fix(c.Request().Context(), id, in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) ChequeBanks(c echo.Context) error {
	banks, err := cheques.Banks(c.Request().Context(), h.d.DB)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, banks)
}

// ChequeDuplicate warns (never blocks) when a cheque was seen before:
// received — same bank and number; given — same number on the same bank place.
func (h *handlers) ChequeDuplicate(c echo.Context) error {
	dup, err := cheques.Duplicate(c.Request().Context(), h.d.DB, c.QueryParam("dir"), strings.TrimSpace(c.QueryParam("bank")),
		qInt(c, "lockerId"), strings.TrimSpace(c.QueryParam("number")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, ChequeDuplicate{Duplicate: dup})
}
