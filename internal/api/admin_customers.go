package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const customerPageSize = 30

func (h *handlers) registerAdminCustomers(g *echo.Group) {
	g.GET("/customers", h.AdminCustomers)
	g.POST("/customers", h.AdminCustomerCreate)
	g.GET("/customers/:id", h.AdminCustomer)
	g.PUT("/customers/:id", h.AdminCustomerUpdate)
	g.DELETE("/customers/:id", h.AdminCustomerDisable)
	g.POST("/customers/:id/activate", h.AdminCustomerReactivate)
	g.POST("/customers/:id/opening", h.AdminCustomerOpening)
	g.POST("/customers/:id/payment", h.AdminCustomerPay)
	g.GET("/customers/:id/statement", h.AdminCustomerStatement)
}

func adminCustomerRow(c customers.Customer) AdminCustomerRow {
	return AdminCustomerRow{ID: c.ID, Name: c.Name, Phone: strOr(c.Phone), Address: strOr(c.Address), CreditLimit: dec2(c.CreditLimit),
		Owed: dec2(c.OutstandingBalance), OldDebt: dec2(c.OpeningUnlinked), Loyalty: c.LoyaltyPoints,
		OverLimit: c.CreditLimit.IsPositive() && c.OutstandingBalance.GreaterThan(c.CreditLimit), Active: c.IsActive}
}

// AdminCustomers lists customers (karots-pos /admin/customers): search on name
// or phone; show=owing | advance | over (over their limit); disabled=1 adds the
// disabled ones. Totals cover every match.
func (h *handlers) AdminCustomers(c echo.Context) error {
	ctx := c.Request().Context()
	q := strings.TrimSpace(c.QueryParam("q"))
	var rows []customers.Customer
	var err error
	if c.QueryParam("disabled") == "1" {
		rows, err = h.t.customers.ListAll(ctx, q)
	} else {
		rows, err = h.t.customers.List(ctx, q)
	}
	if err != nil {
		return err
	}
	show := c.QueryParam("show")
	owed, adv := decimal.Zero, decimal.Zero
	kept := make([]AdminCustomerRow, 0, len(rows))
	for _, cu := range rows {
		r, b := adminCustomerRow(cu), cu.OutstandingBalance
		if (show == "owing" && !b.IsPositive()) || (show == "advance" && !b.IsNegative()) || (show == "over" && !r.OverLimit) {
			continue
		}
		if b.IsPositive() {
			owed = owed.Add(b)
		} else {
			adv = adv.Sub(b)
		}
		kept = append(kept, r)
	}
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	out := AdminCustomerList{Items: kept[min((page-1)*customerPageSize, len(kept)):min(page*customerPageSize, len(kept))],
		Total: len(kept), Page: page, PageSize: customerPageSize, Owed: dec2(owed), Advance: dec2(adv)}
	return c.JSON(http.StatusOK, out)
}

func customerStatement(st *customers.Statement) Statement {
	cu := st.Customer
	out := Statement{Name: cu.Name, Phone: strOr(cu.Phone), Address: strOr(cu.Address), Since: cu.CreatedAt.Format("2006-01-02"),
		Lines: make([]StatementLine, 0, len(st.Entries)), TotalDebit: dec2(st.TotalDebit), TotalCredit: dec2(st.TotalCredit), Balance: dec2(cu.OutstandingBalance)}
	for _, e := range st.Entries {
		out.Lines = append(out.Lines, StatementLine{Date: e.Date.Format(time.RFC3339), Kind: e.Kind, Ref: e.Ref, Debit: dec2(e.Debit), Credit: dec2(e.Credit), Balance: dec2(e.Balance)})
	}
	return out
}

// AdminCustomer is one customer with their last 30 ledger lines, newest first.
func (h *handlers) AdminCustomer(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	st, err := h.t.customers.Statement(c.Request().Context(), id)
	if err != nil {
		return err
	}
	lines := customerStatement(st).Lines
	recent := make([]StatementLine, 0, min(len(lines), 30))
	for i := len(lines) - 1; i >= 0 && len(recent) < 30; i-- {
		recent = append(recent, lines[i])
	}
	return c.JSON(http.StatusOK, AdminCustomerDetail{AdminCustomerRow: adminCustomerRow(st.Customer), Since: st.Customer.CreatedAt.Format("2006-01-02"), Recent: recent})
}

func (h *handlers) AdminCustomerStatement(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	st, err := h.t.customers.Statement(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, customerStatement(st))
}

func bindCustomer(c echo.Context) (customers.CreateInput, error) {
	var req CustomerRequest
	if err := c.Bind(&req); err != nil {
		return customers.CreateInput{}, apperr.BadRequest("invalid request body")
	}
	phone := strings.TrimSpace(req.Phone)
	in := customers.CreateInput{Name: strings.TrimSpace(req.Name), Phone: &phone, Address: optStr(req.Address),
		CreditLimit: strings.TrimSpace(req.CreditLimit), OpeningBalance: strings.TrimSpace(req.OpeningBalance)}
	return in, c.Validate(&in)
}

// AdminCustomerCreate adds a customer. customers.Create reuses a phone match
// (the till wants that); here a repeat is a mistake, so it's a 409.
func (h *handlers) AdminCustomerCreate(c echo.Context) error {
	in, err := bindCustomer(c)
	if err != nil {
		return err
	}
	cu, reused, err := h.t.customers.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	if reused {
		return apperr.Conflict("a customer with this phone number already exists")
	}
	h.audit(c, audit.ActionCreate, "customer", strconv.FormatInt(cu.ID, 10), "created customer "+cu.Name)
	return c.JSON(http.StatusCreated, adminCustomerRow(*cu))
}

func (h *handlers) AdminCustomerUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, err := bindCustomer(c)
	if err != nil {
		return err
	}
	if err := h.t.customers.Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "customer", strconv.FormatInt(id, 10), "updated customer "+in.Name)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminCustomerDisable(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.customers.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "customer", strconv.FormatInt(id, 10), "")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminCustomerReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.customers.Reactivate(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "customer", strconv.FormatInt(id, 10), "reactivated")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminCustomerOpening(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req OpeningRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	before, after, err := h.t.customers.AdjustOpening(c.Request().Context(), id, strings.TrimSpace(req.Opening))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "customer", strconv.FormatInt(id, 10), openingAuditDetail(before.OpeningUnlinked, after.OpeningUnlinked, req.Note))
	return c.NoContent(http.StatusNoContent)
}

// AdminCustomerPay collects a credit repayment (karots-pos CustomerPay): cash
// into a locker or open till; card / online into the named card / bank account,
// or recorded only. The payment and the money move commit together.
func (h *handlers) AdminCustomerPay(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req CustomerPayRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	method, ok := normSupplierMethod(req.Method)
	if !ok {
		return apperr.Validation("invalid payment method")
	}
	dest, err := adminMoneyLoc(method, req.Source)
	if err != nil {
		return err
	}
	if method != "cash" && dest.Kind == cashflow.KindTill {
		return apperr.Validation("a " + method + " payment lands in a card or bank account, not a till")
	}
	in := customers.PaymentInput{Amount: strings.TrimSpace(req.Amount), Method: method, Reference: optStr(req.Reference), Note: optStr(req.Note), ApplyToOpening: req.OldDebtFirst}
	if err := c.Validate(&in); err != nil {
		return err
	}
	ctx := c.Request().Context()
	cu, err := h.t.customers.Get(ctx, id)
	if err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	var res *customers.PaymentResult
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		var err error
		if res, err = h.t.customers.RecordPaymentTx(ctx, tx, id, in, userID); err != nil || dest.Kind == cashflow.KindExternal {
			return err
		}
		mv := cashflow.MoveInput{From: cashflow.External(), To: dest, Amount: res.Amount, Reason: "credit collected: " + cu.Name,
			ReceiptKind: "customer_payment", Party: cu.Name, Ref: &cashflow.Ref{Kind: "customer_payment", ID: res.PaymentID}, ActorID: userID}
		if method != "cash" { // AccountLocationTx is the trust boundary for a non-cash account
			if mv.To, err = cashflow.AccountLocationTx(ctx, tx, dest.ID, method); err != nil {
				return err
			}
			mv.LockerKind = "payment"
		}
		_, err = h.d.Cashflow.MoveTx(ctx, tx, mv)
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, "customer", strconv.FormatInt(id, 10), "credit payment "+in.Amount)
	if dest.Kind == cashflow.KindTill {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusCreated, CreditPaymentResult{PaymentID: res.PaymentID, ReceiptNo: res.ReceiptNo, BalanceAfter: dec2(res.BalanceAfter)})
}
