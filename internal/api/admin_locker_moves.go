package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminLockerMoves(g *echo.Group) {
	g.POST("/lockers/transfer", h.AdminLockerTransfer)
	g.POST("/lockers/:id/adjust", h.AdminLockerAdjust)
	g.POST("/lockers/settle-card", h.AdminSettleCard)
}

func moveResult(c echo.Context, r *cashflow.Receipt) error {
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: &r.ID})
}

// AdminLockerTransfer moves money between two lockers (karots-pos LockerTransfer).
// The source can't go below zero unless it allows it (cashflow's guard → 409).
func (h *handlers) AdminLockerTransfer(c echo.Context) error {
	var in LockerTransferRequest
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid transfer")
	}
	if in.FromID <= 0 || in.ToID <= 0 {
		return apperr.Validation("pick both lockers")
	}
	amt, err := parseAmount(in.Amount, "amount")
	if err != nil {
		return err
	}
	rec, err := h.d.Cashflow.Move(c.Request().Context(), cashflow.MoveInput{From: cashflow.Locker(in.FromID), To: cashflow.Locker(in.ToID),
		Amount: amt, Reason: strings.TrimSpace(in.Note), ActorID: middleware.CurrentUserID(c)})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "locker", strconv.FormatInt(in.FromID, 10), "transferred "+dec2(amt)+" to locker "+strconv.FormatInt(in.ToID, 10))
	return moveResult(c, rec)
}

// AdminLockerAdjust is a bank charge (out, booked as Expense "Bank charges" in
// the same tx), interest earned (in), or a count correction up / down — which
// needs a note saying why (karots-pos LockerAdjust).
func (h *handlers) AdminLockerAdjust(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var in LockerAdjustRequest
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid adjustment")
	}
	amt, err := parseAmount(in.Amount, "amount")
	if err != nil {
		return err
	}
	note := strings.TrimSpace(in.Note)
	var mv cashflow.MoveInput
	switch in.Type {
	case "bank_charge":
		mv = cashflow.MoveInput{From: cashflow.Locker(id), To: cashflow.External(), LockerKind: "bank_charge", ReceiptKind: "bank_charge"}
	case "interest":
		mv = cashflow.MoveInput{From: cashflow.External(), To: cashflow.Locker(id), LockerKind: "interest", ReceiptKind: "interest"}
	case "adjust_up", "adjust_down":
		if note == "" {
			return apperr.Validation("say why the count changed")
		}
		mv = cashflow.MoveInput{From: cashflow.External(), To: cashflow.Locker(id), LockerKind: "adjust", ReceiptKind: "adjust"}
		if in.Type == "adjust_down" {
			mv.From, mv.To = mv.To, mv.From
		}
	default:
		return apperr.Validation("pick what kind of adjustment")
	}
	userID := middleware.CurrentUserID(c)
	mv.Amount, mv.Reason, mv.ActorID = amt, note, userID
	ctx := c.Request().Context()
	var rec *cashflow.Receipt
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if in.Type == "bank_charge" {
			e, err := expenses.NewService(h.d.DB).CreateInTx(ctx, tx, expenses.CreateInput{Category: "Bank charges", Amount: dec2(amt), Description: &note}, userID)
			if err != nil {
				return err
			}
			mv.Ref = &cashflow.Ref{Kind: "expense", ID: e.ID}
		}
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, mv)
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "locker", strconv.FormatInt(id, 10), "locker adjustment ("+in.Type+") "+dec2(amt))
	return moveResult(c, rec)
}

// AdminSettleCard banks a card-machine batch (karots-pos SettleCard): the fee
// leaves the card account as a bank charge booked as Expense "Card settlement
// fee", and the net moves card → bank — so the card drains by the gross. The
// receipt returned is the card → bank one.
func (h *handlers) AdminSettleCard(c echo.Context) error {
	var in SettleCardRequest
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid settlement")
	}
	gross, err := parseAmount(in.Gross, "amount settled")
	if err != nil {
		return err
	}
	fee := decimal.Zero
	if strings.TrimSpace(in.Fee) != "" && strings.TrimSpace(in.Fee) != "0" {
		if fee, err = parseAmount(in.Fee, "service fee"); err != nil {
			return err
		}
	}
	if fee.GreaterThanOrEqual(gross) {
		return apperr.Validation("the fee must be less than the amount settled")
	}
	ctx := c.Request().Context()
	card, err := h.t.lockers.Get(ctx, in.CardID)
	if err != nil || card.Kind != lockers.KindCard || !card.IsActive {
		return apperr.Validation("pick a card account")
	}
	bank, err := h.t.lockers.Get(ctx, in.BankID)
	if err != nil || bank.Kind != lockers.KindBank || !bank.IsActive {
		return apperr.Validation("pick a bank account")
	}
	userID := middleware.CurrentUserID(c)
	var rec *cashflow.Receipt
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if fee.IsPositive() {
			desc := "card settlement: " + card.Name
			e, err := expenses.NewService(h.d.DB).CreateInTx(ctx, tx, expenses.CreateInput{Category: "Card settlement fee", Amount: dec2(fee), Description: &desc}, userID)
			if err != nil {
				return err
			}
			if _, err := h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Locker(card.ID), To: cashflow.External(), Amount: fee,
				Reason: desc + " fee", ReceiptKind: "expense", LockerKind: "bank_charge", Ref: &cashflow.Ref{Kind: "expense", ID: e.ID}, ActorID: userID}); err != nil {
				return err
			}
		}
		rec, err = h.d.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Locker(card.ID), To: cashflow.Locker(bank.ID), Amount: gross.Sub(fee),
			Reason: "card settlement: " + card.Name + " → " + bank.Name, ReceiptKind: "transfer", ActorID: userID})
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "locker", strconv.FormatInt(card.ID, 10), "settled card account "+card.Name+" into "+bank.Name)
	return moveResult(c, rec)
}
