package cheques

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
)

// ReceiveTx records a cheque taken from a customer and puts its amount into
// Cheques in hand. The caller has already counted the payment (sale paid /
// customer balance down) like any other method.
func ReceiveTx(ctx context.Context, tx *sqlx.Tx, cf *cashflow.Service, l Line, k Link, actor int64) (*Cheque, error) {
	if k.CustomerID <= 0 {
		return nil, apperr.Validation("a cheque must come from a named customer")
	}
	if err := checkLine(l); err != nil {
		return nil, err
	}
	c, err := insert(ctx, tx, "in", l, nil, k, actor)
	if err != nil {
		return nil, err
	}
	inHand, err := lockers.ChequesInHandTx(ctx, tx)
	if err != nil {
		return nil, apperr.Internal("failed to find Cheques in hand", err)
	}
	kind, ref := "customer_payment", (*cashflow.Ref)(nil)
	switch {
	case k.SaleID != nil:
		kind, ref = "payment", &cashflow.Ref{Kind: "sale", ID: *k.SaleID}
	case k.CustomerPaymentID != nil:
		ref = &cashflow.Ref{Kind: "customer_payment", ID: *k.CustomerPaymentID}
	}
	if _, err := cf.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.External(), To: cashflow.Locker(inHand),
		Amount: l.Amount, Reason: "cheque " + l.Number, ReceiptKind: kind, Ref: ref, Party: k.Party, ActorID: actor}); err != nil {
		return nil, err
	}
	return c, nil
}

// GiveTx records a cheque the shop wrote on one of its bank places. No money
// leaves the bank until it clears.
func GiveTx(ctx context.Context, tx *sqlx.Tx, l Line, k Link, actor int64) (*Cheque, error) {
	if err := checkLine(l); err != nil {
		return nil, err
	}
	if _, err := activeBank(ctx, tx, l.LockerID); err != nil {
		return nil, err
	}
	return insert(ctx, tx, "out", l, &l.LockerID, k, actor)
}

func checkLine(l Line) error {
	if l.Number == "" {
		return apperr.Validation("enter the cheque number")
	}
	if !l.Amount.IsPositive() {
		return apperr.Validation("cheque amount must be greater than zero")
	}
	if l.Date.IsZero() {
		return apperr.Validation("enter the cheque date")
	}
	return nil
}

func insert(ctx context.Context, tx *sqlx.Tx, dir string, l Line, locker *int64, k Link, actor int64) (*Cheque, error) {
	var id int64
	err := tx.GetContext(ctx, &id, `
		INSERT INTO cheques (direction, bank, locker_id, number, amount, cheque_date, note, customer_id, supplier_id,
		                     sale_id, payment_id, customer_payment_id, supplier_payment_id, expense_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`,
		dir, l.Bank, locker, l.Number, l.Amount, l.Date.Format("2006-01-02"), l.Note, nz(k.CustomerID), nz(k.SupplierID),
		k.SaleID, k.PaymentID, k.CustomerPaymentID, k.SupplierPaymentID, k.ExpenseID, nz(actor))
	if err != nil {
		return nil, apperr.Internal("failed to record cheque", err)
	}
	if err := audit.NewRepository(tx).Insert(ctx, actor, audit.ActionCreate, "cheque", idStr(id),
		map[string]string{"in": "Received", "out": "Written"}[dir]+" · "+money.Display(l.Amount)); err != nil {
		return nil, apperr.Internal("failed to record cheque history", err)
	}
	return get(ctx, tx, id)
}

// activeBank returns the name of an active bank place, or a Validation error.
func activeBank(ctx context.Context, q appdb.Queryer, id int64) (string, error) {
	lk, err := lockers.NewRepository(q).Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", apperr.Validation("pick one of the shop's bank places")
	}
	if err != nil {
		return "", apperr.Internal("failed to load locker", err)
	}
	if lk.Kind != lockers.KindBank || !lk.IsActive {
		return "", apperr.Validation(lk.Name + " is not an active bank place")
	}
	return lk.Name, nil
}

// nz turns 0 into NULL.
func nz(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

// SaleLeg is the till's cheque seam (sales.WithChequeLeg): each cheque tender
// line becomes a received cheque, linked to its sale and payment line, with its
// money in Cheques in hand — all on the sale's own tx.
func SaleLeg(cf *cashflow.Service) sales.ChequeLeg {
	return func(ctx context.Context, tx *sqlx.Tx, ev sales.ChequeTaken) error {
		ls, _, err := ParseLines([]LineInput{{Bank: ev.Cheque.Bank, Number: ev.Cheque.Number, Amount: ev.Amount.String(),
			Date: ev.Cheque.Date, Note: ev.Cheque.Note}}, "in")
		if err != nil {
			return err
		}
		_, err = ReceiveTx(ctx, tx, cf, ls[0], Link{CustomerID: ev.CustomerID, SaleID: &ev.SaleID, PaymentID: &ev.PaymentID, Party: ev.Party}, ev.ActorID)
		return err
	}
}
