package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// adminMoneyLoc resolves an admin form's source/destination. Cash has to name
// one; card and online may be outside the shop.
// The place must fit the method: cash a till or cash locker, card / online a
// bank account, never a card-machine account (cashflow.RequirePlace).
func (h *handlers) adminMoneyLoc(ctx context.Context, method, source string) (cashflow.Location, error) {
	if method == "cash" && strings.TrimSpace(source) == "" {
		return cashflow.Location{}, apperr.Validation("pick where the cash comes from or goes to")
	}
	loc, err := parseLocation(source)
	if err != nil {
		return loc, err
	}
	return loc, h.d.Cashflow.RequirePlace(ctx, loc, method)
}

// optAmount parses a blank-able, non-negative amount.
func optAmount(s, what string) (decimal.Decimal, error) {
	if s = strings.TrimSpace(s); s == "" {
		return decimal.Zero, nil
	}
	v, err := money.Parse(s)
	if err != nil || v.IsNegative() {
		return decimal.Zero, apperr.Validation("invalid " + what)
	}
	return v, nil
}

// supplierPayInput is the admin split: exact allocations / advance /
// old debt when any is given, else one amount waterfalled across what's owed.
func supplierPayInput(req AdminSupplierPayRequest, invoices []purchases.Purchase, opening decimal.Decimal) (supplierpay.PayInput, error) {
	explicit := strings.TrimSpace(req.Advance) != "" || strings.TrimSpace(req.OldDebt) != ""
	for _, a := range req.Allocations {
		explicit = explicit || strings.TrimSpace(a.Amount) != ""
	}
	if !explicit {
		amt, err := money.Parse(strings.TrimSpace(req.Amount))
		if err != nil || !amt.IsPositive() {
			return supplierpay.PayInput{}, apperr.Validation("enter the amount paid")
		}
		return supplierpay.Distribute(invoices, opening, amt, req.Mode), nil
	}
	var in supplierpay.PayInput
	open := map[int64]bool{}
	for _, p := range invoices {
		open[p.ID] = true
	}
	for _, a := range req.Allocations {
		amt, err := optAmount(a.Amount, "allocation amount")
		if err != nil {
			return in, err
		}
		if amt.IsZero() {
			continue
		}
		if !open[a.PurchaseID] {
			return in, apperr.Validation("that invoice is not open")
		}
		in.Allocations = append(in.Allocations, supplierpay.Alloc{PurchaseID: a.PurchaseID, Amount: amt})
	}
	var err error
	if in.Unallocated, err = optAmount(req.Advance, "advance"); err != nil {
		return in, err
	}
	if in.Opening, err = optAmount(req.OldDebt, "old-debt amount"); err != nil {
		return in, err
	}
	return in, nil
}

// AdminSupplierPay pays a supplier from any locker or open till (or from
// outside the shop by card / online); the payment and the money leaving commit
// together.
func (h *handlers) AdminSupplierPay(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AdminSupplierPayRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	src, err := h.adminGiven(c.Request().Context(), req.Method, req.Source, req.Cheques)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	sup, err := suppliers.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	inv, err := supplierpay.NewService(h.d.DB).OpenInvoices(ctx, id)
	if err != nil {
		return err
	}
	in, err := supplierPayInput(req, inv, sup.OpeningUnlinked)
	if err != nil {
		return err
	}
	in.Method, in.Reference, in.Note = src.Method, strings.TrimSpace(req.Reference), strings.TrimSpace(req.Note)
	userID := middleware.CurrentUserID(c)
	var rec *cashflow.Receipt
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		rec, err = h.paySupplierTx(ctx, tx, id, sup.Name, in, src, userID)
		return err
	})
	if err != nil {
		return err
	}
	total := in.Unallocated.Add(in.Opening)
	for _, a := range in.Allocations {
		total = total.Add(a.Amount)
	}
	h.audit(c, audit.ActionPayment, "supplier", strconv.FormatInt(id, 10), "paid "+money.Display(total)+" ("+paidHow(src)+")")
	return h.adminMoneyDone(ctx, c, src.Loc, rec)
}

// AdminSupplierRefund records money a supplier hands back (capped at the credit
// they hold by supplierpay).
func (h *handlers) AdminSupplierRefund(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AdminMoneyRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	method, ok := normSupplierMethod(req.Method)
	if !ok {
		return apperr.Validation("refund method must be cash, card or online")
	}
	amt, err := money.Parse(strings.TrimSpace(req.Amount))
	if err != nil || !amt.IsPositive() {
		return apperr.Validation("enter the amount received")
	}
	dest, err := h.adminMoneyLoc(c.Request().Context(), method, req.Source)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	sup, err := suppliers.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	var rec *cashflow.Receipt
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		r, err := supplierpay.NewService(h.d.DB).RefundTx(ctx, tx, id, supplierpay.RefundInput{Amount: amt, Method: method,
			Reference: strings.TrimSpace(req.Reference), Note: strings.TrimSpace(req.Note)}, userID)
		if err != nil || dest.Kind == cashflow.KindExternal {
			return err
		}
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.External(), To: dest, Amount: r.Amount,
			Reason: "refund from " + sup.Name, ReceiptKind: "supplier_refund", Party: sup.Name,
			Ref: &cashflow.Ref{Kind: "supplier_refund", ID: r.RefundID}, ActorID: userID})
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, "supplier", strconv.FormatInt(id, 10), "refund received "+money.Display(amt)+" ("+method+")")
	return h.adminMoneyDone(ctx, c, dest, rec)
}

// adminMoneyDone pops the drawer when cash went through a till and answers with the receipt, if one was made.
func (h *handlers) adminMoneyDone(ctx context.Context, c echo.Context, loc cashflow.Location, rec *cashflow.Receipt) error {
	if loc.Kind == cashflow.KindTill {
		h.d.KickDrawer(ctx)
	}
	if rec == nil {
		return c.JSON(http.StatusCreated, ReceiptRef{})
	}
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: &rec.ID})
}
