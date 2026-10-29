package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/payout"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

func supplierRow(s suppliers.Supplier) SupplierRow {
	return SupplierRow{ID: s.ID, Name: s.Name, Phone: strOr(s.Phone), Contact: strOr(s.ContactPerson),
		Owed: dec2(s.OutstandingBalance), OldDebt: dec2(s.OpeningUnlinked), CreditDays: s.CreditDays}
}

// normSupplierMethod is karots-pos's: blank means cash.
func normSupplierMethod(m string) (string, bool) {
	switch m {
	case "cash", "card", "online":
		return m, true
	case "":
		return "cash", true
	}
	return "", false
}

// supplierMoneyLoc: cash always touches the counter (drawer or a cashier locker);
// card/online touch a locker only when one is picked, else it's outside the shop.
// The place must fit the method: cash a drawer or cash locker, card / online a
// bank account, never a card-machine account (cashflow.RequirePlace).
func (h *handlers) supplierMoneyLoc(c echo.Context, method string, lockerID int64) (cashflow.Location, error) {
	if method != "cash" && lockerID == 0 {
		return cashflow.External(), nil
	}
	loc, err := h.counterSource(c, lockerID)
	if err != nil {
		return loc, err
	}
	return loc, h.d.Cashflow.RequirePlace(c.Request().Context(), loc, method)
}

// paySupplierTx books a supplier payment and pays it from src — the money
// leaving the drawer/locker now, or the cheques written for it — in the
// caller's tx (payout.SupplierTx). No slip when nothing left the counter.
func (h *handlers) paySupplierTx(ctx context.Context, tx *sqlx.Tx, supID int64, name string, in supplierpay.PayInput, src payout.Source, userID int64) (*cashflow.Receipt, error) {
	return h.payout().SupplierTx(ctx, tx, supID, name, in, src, userID)
}

func (h *handlers) CounterSuppliers(c echo.Context) error {
	rows, err := suppliers.NewService(h.d.DB).List(c.Request().Context(), strings.TrimSpace(c.QueryParam("q")))
	if err != nil {
		return err
	}
	out := make([]SupplierRow, 0, len(rows))
	for _, s := range rows {
		out = append(out, supplierRow(s))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) CounterSupplier(c echo.Context) error {
	id, err := pathID(c)
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
	drafts, err := purchases.NewService(h.d.DB).ListDrafts(ctx)
	if err != nil {
		return err
	}
	v := SupplierView{SupplierRow: supplierRow(*sup), Invoices: []SupplierInvoice{}, Orders: []SupplierOrder{}}
	for _, d := range drafts {
		if d.SupplierID == id {
			v.Orders = append(v.Orders, SupplierOrder{ID: d.ID, Date: d.CreatedAt.Format(time.RFC3339), Total: dec2(d.Total), Notes: strOr(d.Notes)})
		}
	}
	for _, p := range inv {
		row := SupplierInvoice{ID: p.ID, InvoiceNo: strOr(p.InvoiceNo), Date: p.CreatedAt.Format(time.RFC3339), Total: dec2(p.Total), Balance: dec2(p.Balance())}
		if p.DueDate != nil {
			row.DueDate = p.DueDate.Format("2006-01-02")
		}
		v.Invoices = append(v.Invoices, row)
	}
	return c.JSON(http.StatusOK, v)
}

// CounterSupplierPay pays a supplier at the counter; the payment ledger and the
// money leaving the drawer/locker commit together (karots-pos paySupplierTx).
func (h *handlers) CounterSupplierPay(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req SupplierPayRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	amt, err := money.Parse(strings.TrimSpace(req.Amount))
	if err != nil || !amt.IsPositive() {
		return apperr.Validation("enter the amount paid")
	}
	src, err := h.counterGiven(c, req.Method, req.LockerID, req.Cheques)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	sp := supplierpay.NewService(h.d.DB)
	sup, err := suppliers.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	inv, err := sp.OpenInvoices(ctx, id)
	if err != nil {
		return err
	}
	in := supplierpay.Distribute(inv, sup.OpeningUnlinked, amt, req.Mode)
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
	h.audit(c, audit.ActionPayment, "supplier", strconv.FormatInt(id, 10), "paid "+money.Display(amt)+" ("+paidHow(src)+") at the counter")
	return h.counterSlip(c, src.Loc, req.CounterMoneyRequest, rec)
}

// CounterSupplierRefund records money a supplier hands back.
func (h *handlers) CounterSupplierRefund(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req SupplierRefundRequest
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
	dest, err := h.supplierMoneyLoc(c, method, req.LockerID)
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
	h.audit(c, audit.ActionPayment, "supplier", strconv.FormatInt(id, 10), "refund received "+money.Display(amt)+" ("+method+") at the counter")
	return h.counterSlip(c, dest, req.CounterMoneyRequest, rec)
}
