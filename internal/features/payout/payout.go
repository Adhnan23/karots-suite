// Package payout is the one way the shop gives money: a supplier payment or an
// expense, paid now from a till / locker, from outside the shop, or by cheques
// written on the shop's bank places. A cheque books the payment or expense at
// once but moves no money — the bank pays it when it clears (package cheques).
package payout

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// MethodCheque pays by one or more cheques (Source.Cheques).
const MethodCheque = "cheque"

// Source is where given money comes from: Loc (a till, a locker, or External =
// outside the shop) paid by Method now, or — Method "cheque" — the cheques
// written for it, which must add up to the amount.
type Source struct {
	Loc     cashflow.Location
	Method  string
	Cheques []cheques.Line
}

// IsCheque reports whether the money goes by cheque.
func (s Source) IsCheque() bool { return s.Method == MethodCheque }

// Service pays suppliers and expenses.
type Service struct {
	db *sqlx.DB
	cf *cashflow.Service
}

func NewService(db *sqlx.DB, cf *cashflow.Service) *Service { return &Service{db: db, cf: cf} }

// chequesTotal checks the cheques add up to what is being paid.
func chequesTotal(ls []cheques.Line, want decimal.Decimal) error {
	if len(ls) == 0 {
		return apperr.Validation("add the cheque details")
	}
	sum := decimal.Zero
	for _, l := range ls {
		sum = sum.Add(l.Amount)
	}
	if !sum.Equal(want) {
		return apperr.Validation("the cheques add up to " + money.Display(sum) + ", the payment is " + money.Display(want))
	}
	return nil
}

// SupplierTx books a supplier payment in the caller's tx and pays it from src:
// money leaves the place now (none for outside the shop), or the cheques are
// written for it. Returns the money receipt when money left a place now.
func (s *Service) SupplierTx(ctx context.Context, tx *sqlx.Tx, supplierID int64, supplierName string, in supplierpay.PayInput, src Source, userID int64) (*cashflow.Receipt, error) {
	if src.IsCheque() {
		in.Method = MethodCheque
	}
	res, err := supplierpay.NewService(s.db).PayTx(ctx, tx, supplierID, in, userID)
	if err != nil {
		return nil, err
	}
	if src.IsCheque() {
		if err := chequesTotal(src.Cheques, res.Total); err != nil {
			return nil, err
		}
		for _, l := range src.Cheques {
			if _, err := cheques.GiveTx(ctx, tx, l, cheques.Link{SupplierID: supplierID, SupplierPaymentID: &res.PaymentID, Party: supplierName}, userID); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if src.Loc.Kind == cashflow.KindExternal {
		return nil, nil
	}
	return s.cf.MoveTx(ctx, tx, cashflow.MoveInput{From: src.Loc, To: cashflow.External(), Amount: res.Total,
		Reason: "supplier payment: " + supplierName, ReceiptKind: "supplier_payment", Party: supplierName,
		Ref: &cashflow.Ref{Kind: "supplier_payment", ID: res.PaymentID}, ActorID: userID})
}

// ExpenseTx books an expense in the caller's tx and pays it from src (see
// SupplierTx). Returns the expense id and the money receipt, if money moved now.
func (s *Service) ExpenseTx(ctx context.Context, tx *sqlx.Tx, in expenses.CreateInput, src Source, userID int64) (int64, *cashflow.Receipt, error) {
	if !src.IsCheque() {
		// Never out of a card-machine account: that money leaves only through Settle card.
		if err := cashflow.RequirePlaceTx(ctx, tx, src.Loc, ""); err != nil {
			return 0, nil, err
		}
	}
	e, err := expenses.NewService(s.db).CreateInTx(ctx, tx, in, userID)
	if err != nil {
		return 0, nil, err
	}
	rec, err := s.payExpense(ctx, tx, e.ID, expenseReason(e), e.Amount, src, userID)
	return e.ID, rec, err
}

// PayAgainTx pays what is still unpaid on an expense (its cheque bounced or was
// cancelled) from src, in the caller's tx. The expense stays one expense.
func (s *Service) PayAgainTx(ctx context.Context, tx *sqlx.Tx, expenseID int64, src Source, userID int64) (*cashflow.Receipt, error) {
	if !src.IsCheque() {
		if err := cashflow.RequirePlaceTx(ctx, tx, src.Loc, ""); err != nil {
			return nil, err
		}
	}
	var e expenses.Expense
	if err := tx.GetContext(ctx, &e, `SELECT * FROM expenses WHERE id = $1 FOR UPDATE`, expenseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("expense")
		}
		return nil, apperr.Internal("failed to load expense", err)
	}
	if !e.Unpaid.IsPositive() {
		return nil, apperr.Validation("this expense is already paid")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE expenses SET unpaid = 0 WHERE id = $1`, expenseID); err != nil {
		return nil, apperr.Internal("failed to pay the expense", err)
	}
	return s.payExpense(ctx, tx, e.ID, expenseReason(&e), e.Unpaid, src, userID)
}

func (s *Service) payExpense(ctx context.Context, tx *sqlx.Tx, id int64, reason string, amount decimal.Decimal, src Source, userID int64) (*cashflow.Receipt, error) {
	if src.IsCheque() {
		if err := chequesTotal(src.Cheques, amount); err != nil {
			return nil, err
		}
		for _, l := range src.Cheques {
			if _, err := cheques.GiveTx(ctx, tx, l, cheques.Link{ExpenseID: &id, Party: reason}, userID); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if src.Loc.Kind == cashflow.KindExternal {
		return nil, nil
	}
	return s.cf.MoveTx(ctx, tx, cashflow.MoveInput{From: src.Loc, To: cashflow.External(), Amount: amount,
		Reason: reason, ReceiptKind: "expense", Ref: &cashflow.Ref{Kind: "expense", ID: id}, ActorID: userID})
}

func expenseReason(e *expenses.Expense) string {
	r := strings.TrimSpace(e.Category)
	if e.Description != nil && *e.Description != "" {
		r += " - " + *e.Description
	}
	return r
}
