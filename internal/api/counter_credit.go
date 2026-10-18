package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/receiptimg"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

var methodLabel = map[string]string{"cash": "Cash", "card": "Card", "online": "Online"}

// CounterCreditPayment collects money a customer owes (or takes an advance).
// The ledger row and the money move commit together: cash into this drawer,
// card/online into the picked account (karots-pos cashier CreditPay).
func (h *handlers) CounterCreditPayment(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req CreditPaymentRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := customers.PaymentInput{Amount: req.Amount, Method: req.Method, AccountID: req.AccountID}
	tender, isPlugin := paymentTender(req.Method)
	in.Plugin = isPlugin
	if req.Reference != "" {
		in.Reference = &req.Reference
	}
	if n := strings.TrimSpace(req.Note); n != "" {
		in.Note = &n
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	ctx := c.Request().Context()
	cust, err := h.t.customers.Get(ctx, id)
	if err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	var res *customers.PaymentResult
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		var err error
		if res, err = h.t.customers.RecordPaymentTx(ctx, tx, id, in, userID); err != nil {
			return err
		}
		if isPlugin { // the plugin books where the money landed
			return tender.SettleTx(ctx, tx, plugin.TenderSettle{Amount: res.Amount, Meta: req.Meta, ActorID: userID, CustomerPaymentID: res.PaymentID, CustomerID: id})
		}
		mv := cashflow.MoveInput{From: cashflow.External(), Amount: res.Amount, Reason: "credit collected: " + cust.Name,
			ReceiptKind: "customer_payment", Party: cust.Name, Ref: &cashflow.Ref{Kind: "customer_payment", ID: res.PaymentID}, ActorID: userID}
		if res.Method == "cash" {
			mv.To = cashflow.Till(userID)
		} else { // card/online: RecordPaymentTx already refused a missing account
			if mv.To, err = cashflow.AccountLocationTx(ctx, tx, in.AccountID, res.Method); err != nil {
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
	h.audit(c, audit.ActionPayment, "customer", strconv.FormatInt(id, 10), "credit collected "+req.Amount+" from "+cust.Name)
	if res.Method == "cash" && !req.DeferDrawerKick {
		h.d.KickDrawer(ctx)
	}
	return c.JSON(http.StatusCreated, CreditPaymentResult{PaymentID: res.PaymentID, ReceiptNo: res.ReceiptNo, BalanceAfter: dec2(res.BalanceAfter)})
}

// CounterAdvanceRefund hands a customer's advance back in cash from this drawer.
func (h *handlers) CounterAdvanceRefund(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AdvanceRefundRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	cust, err := h.t.customers.Get(ctx, id)
	if err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	till := cashflow.Till(userID)
	var rec *cashflow.Receipt
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		res, err := h.t.customers.RefundAdvanceTx(ctx, tx, id, req.Amount, userID)
		if err != nil {
			return err
		}
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: till, To: cashflow.External(), Amount: res.Amount,
			Reason: "advance refunded: " + cust.Name, ReceiptKind: "refund", Party: cust.Name,
			Ref: &cashflow.Ref{Kind: "customer_payment", ID: res.PaymentID}, ActorID: userID})
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, "customer", strconv.FormatInt(id, 10), "advance refunded "+dec2(rec.Amount)+" to "+cust.Name)
	return h.afterCounterMove(ctx, c, till, CounterMoneyRequest{DeferDrawerKick: req.DeferDrawerKick}, rec)
}

// CounterPrintDebtSlip prints (or reprints) a credit-payment slip. ?kick=1 folds
// the drawer pulse in (auto-print right after a cash collection).
func (h *handlers) CounterPrintDebtSlip(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	r, err := h.t.customers.GetPayment(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	slip := escpos.DebtSlip{ReceiptNo: r.ReceiptNo, Date: datetime.DateTime(r.CreatedAt), CustomerName: r.CustomerName,
		CustomerPhone: strOr(r.CustomerPhone), Method: methodOr(r.Method), CashierName: strOr(r.CashierName),
		Amount: r.Amount, BalanceBefore: r.BalanceBefore, BalanceAfter: r.BalanceAfter}
	payload := escpos.DebtDocument(slip, *cfg, receiptimg.SlipOptions(ctx, cfg, nil))
	if ok, err := previewSlip(c, cfg, payload); ok {
		return err
	}
	if c.QueryParam("kick") == "1" {
		payload = append(escpos.DrawerKick(*cfg), payload...)
	}
	if err := escpos.Send(ctx, h.receiptQueue(c, cfg), payload); err != nil {
		log.Printf("debt slip %d: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}

func methodOr(m string) string {
	if l, ok := methodLabel[m]; ok {
		return l
	}
	return m
}
