package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const supplierPageSize = 30

func (h *handlers) registerAdminSuppliers(g *echo.Group) {
	g.GET("/suppliers", h.AdminSuppliers)
	g.POST("/suppliers", h.AdminSupplierCreate)
	g.GET("/suppliers/:id", h.AdminSupplier)
	g.PUT("/suppliers/:id", h.AdminSupplierUpdate)
	g.DELETE("/suppliers/:id", h.AdminSupplierDisable)
	g.POST("/suppliers/:id/activate", h.AdminSupplierReactivate)
	g.POST("/suppliers/:id/opening", h.AdminSupplierOpening)
	g.POST("/suppliers/:id/reassign", h.AdminSupplierReassign)
	g.GET("/suppliers/:id/statement", h.AdminSupplierStatement)
	g.POST("/suppliers/:id/payment", h.AdminSupplierPay)
	g.POST("/suppliers/:id/refund", h.AdminSupplierRefund)
}

func adminSupplierRow(s suppliers.Supplier) AdminSupplierRow {
	return AdminSupplierRow{ID: s.ID, Name: s.Name, Contact: strOr(s.ContactPerson), Phone: strOr(s.Phone), Address: strOr(s.Address),
		CreditDays: s.CreditDays, Owed: dec2(s.OutstandingBalance), OldDebt: dec2(s.OpeningUnlinked), Active: s.IsActive}
}

// AdminSuppliers lists suppliers: search on name,
// contact or phone; show=owing (we owe them) | credit (they owe us); disabled=1
// adds the disabled ones. Totals cover every match, not just the page.
func (h *handlers) AdminSuppliers(c echo.Context) error {
	ctx := c.Request().Context()
	svc := suppliers.NewService(h.d.DB)
	q := strings.TrimSpace(c.QueryParam("q"))
	var rows []suppliers.Supplier
	var err error
	if c.QueryParam("disabled") == "1" {
		rows, err = svc.ListAll(ctx, q)
	} else {
		rows, err = svc.List(ctx, q)
	}
	if err != nil {
		return err
	}
	show := c.QueryParam("show")
	owed, credit := decimal.Zero, decimal.Zero
	kept := rows[:0]
	for _, s := range rows {
		b := s.OutstandingBalance
		if (show == "owing" && !b.IsPositive()) || (show == "credit" && !b.IsNegative()) {
			continue
		}
		if b.IsPositive() {
			owed = owed.Add(b)
		} else {
			credit = credit.Sub(b)
		}
		kept = append(kept, s)
	}
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	out := AdminSupplierList{Items: []AdminSupplierRow{}, Total: len(kept), Page: page, PageSize: supplierPageSize, Owed: dec2(owed), Credit: dec2(credit)}
	for _, s := range kept[min((page-1)*supplierPageSize, len(kept)):min(page*supplierPageSize, len(kept))] {
		out.Items = append(out.Items, adminSupplierRow(s))
	}
	return c.JSON(http.StatusOK, out)
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// AdminSupplier is one supplier's page: open invoices, orders not yet in,
// payments and refunds, and how many products default to them.
func (h *handlers) AdminSupplier(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	sup, err := suppliers.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	sp := supplierpay.NewService(h.d.DB)
	inv, err := sp.OpenInvoices(ctx, id)
	if err != nil {
		return err
	}
	pays, err := sp.History(ctx, id)
	if err != nil {
		return err
	}
	refunds, err := sp.RefundHistory(ctx, id)
	if err != nil {
		return err
	}
	drafts, err := purchases.NewService(h.d.DB).ListDrafts(ctx)
	if err != nil {
		return err
	}
	n, err := h.t.products.CountByPreferredSupplier(ctx, id)
	if err != nil {
		return err
	}
	out := AdminSupplierDetail{AdminSupplierRow: adminSupplierRow(*sup), Invoices: []SupplierInvoice{}, Orders: []SupplierOrder{}, Money: []SupplierMoneyRow{}, Products: n}
	for _, p := range inv {
		row := SupplierInvoice{ID: p.ID, InvoiceNo: strOr(p.InvoiceNo), Date: p.CreatedAt.Format(time.RFC3339), Total: dec2(p.Total), Balance: dec2(p.Balance())}
		if p.DueDate != nil {
			row.DueDate = p.DueDate.Format("2006-01-02")
		}
		out.Invoices = append(out.Invoices, row)
	}
	for _, d := range drafts {
		if d.SupplierID == id {
			out.Orders = append(out.Orders, SupplierOrder{ID: d.ID, Date: d.CreatedAt.Format(time.RFC3339), Total: dec2(d.Total), Notes: strOr(d.Notes)})
		}
	}
	for _, p := range pays {
		out.Money = append(out.Money, SupplierMoneyRow{Kind: "payment", Date: p.CreatedAt.Format(time.RFC3339), Method: p.Method,
			Reference: strOr(p.Reference), Note: strOr(p.Note), By: strOr(p.PaidByName), Amount: dec2(p.Amount)})
	}
	for _, r := range refunds {
		out.Money = append(out.Money, SupplierMoneyRow{Kind: "refund", Date: r.CreatedAt.Format(time.RFC3339), Method: r.Method,
			Reference: strOr(r.Reference), Note: strOr(r.Note), By: strOr(r.ReceivedByName), Amount: dec2(r.Amount)})
	}
	// RFC3339 in one zone: string order is time order.
	slices.SortStableFunc(out.Money, func(a, b SupplierMoneyRow) int { return strings.Compare(b.Date, a.Date) })
	return c.JSON(http.StatusOK, out)
}

func bindSupplier(c echo.Context) (suppliers.CreateInput, error) {
	var req SupplierRequest
	if err := c.Bind(&req); err != nil {
		return suppliers.CreateInput{}, apperr.BadRequest("invalid request body")
	}
	in := suppliers.CreateInput{Name: strings.TrimSpace(req.Name), ContactPerson: optStr(req.Contact), Phone: optStr(req.Phone),
		Address: optStr(req.Address), CreditDays: req.CreditDays, OpeningBalance: strings.TrimSpace(req.OpeningBalance)}
	return in, c.Validate(&in)
}

// AdminSupplierCreate adds a supplier; an active one with the same name is a 409.
// The opening is what was already owed.
func (h *handlers) AdminSupplierCreate(c echo.Context) error {
	in, err := bindSupplier(c)
	if err != nil {
		return err
	}
	s, err := suppliers.NewService(h.d.DB).Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "supplier", strconv.FormatInt(s.ID, 10), "created supplier "+s.Name)
	return c.JSON(http.StatusCreated, adminSupplierRow(*s))
}

func (h *handlers) AdminSupplierUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, err := bindSupplier(c)
	if err != nil {
		return err
	}
	if err := suppliers.NewService(h.d.DB).Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier", strconv.FormatInt(id, 10), "updated supplier "+in.Name)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminSupplierDisable(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := suppliers.NewService(h.d.DB).Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "supplier", strconv.FormatInt(id, 10), "")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminSupplierReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := suppliers.NewService(h.d.DB).Reactivate(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier", strconv.FormatInt(id, 10), "reactivated")
	return c.NoContent(http.StatusNoContent)
}

// AdminSupplierOpening corrects the old (pre-system) debt; the part backed by
// invoices, payments and returns is never touched. Audited before → after.
func (h *handlers) AdminSupplierOpening(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req OpeningRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	before, after, err := suppliers.NewService(h.d.DB).AdjustOpening(c.Request().Context(), id, strings.TrimSpace(req.Opening))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier", strconv.FormatInt(id, 10), openingAuditDetail(before.OpeningUnlinked, after.OpeningUnlinked, req.Note))
	return c.NoContent(http.StatusNoContent)
}

// openingAuditDetail is the audit line for an opening correction.
func openingAuditDetail(before, after decimal.Decimal, note string) string {
	d := "opening balance " + money.Display(before) + " → " + money.Display(after)
	if note = strings.TrimSpace(note); note != "" {
		d += " — " + note
	}
	return d
}

// AdminSupplierReassign moves every product defaulting to this supplier to
// another one (a distributor change); stock, prices and history stay put.
func (h *handlers) AdminSupplierReassign(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ReassignRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if req.ToID <= 0 || req.ToID == id {
		return apperr.Validation("pick a supplier to move to")
	}
	ctx := c.Request().Context()
	to, err := suppliers.NewService(h.d.DB).Get(ctx, req.ToID)
	if err != nil {
		return err
	}
	if !to.IsActive {
		return apperr.Validation("that supplier is disabled")
	}
	n, err := h.t.products.ReassignPreferredSupplier(ctx, id, req.ToID)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "supplier", strconv.FormatInt(id, 10), "moved "+strconv.FormatInt(n, 10)+" products to supplier "+strconv.FormatInt(req.ToID, 10))
	return c.JSON(http.StatusOK, CountResult{Count: int(n)})
}

func (h *handlers) AdminSupplierStatement(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	st, err := suppliers.NewService(h.d.DB).Statement(c.Request().Context(), id)
	if err != nil {
		return err
	}
	s := st.Supplier
	out := Statement{Name: s.Name, Phone: strOr(s.Phone), Address: strOr(s.Address), Contact: strOr(s.ContactPerson), Since: s.CreatedAt.Format("2006-01-02"),
		Lines: make([]StatementLine, 0, len(st.Entries)), TotalDebit: dec2(st.TotalDebit), TotalCredit: dec2(st.TotalCredit), Balance: dec2(s.OutstandingBalance)}
	for _, e := range st.Entries {
		out.Lines = append(out.Lines, StatementLine{Date: e.Date.Format(time.RFC3339), Kind: e.Kind, Ref: e.Ref, Debit: dec2(e.Debit), Credit: dec2(e.Credit), Balance: dec2(e.Balance)})
	}
	return c.JSON(http.StatusOK, out)
}
