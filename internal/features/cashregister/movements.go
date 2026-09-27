// Package cashregister manages daily cash-drawer sessions. Exactly one session
// per cashier can be open at a time (enforced by a partial unique index).
//
// The drawer is counted by denomination (how many of each note/coin) at open and
// close. Every cash event during a session — opening float, mid-shift
// withdrawals and pay-ins, credit collected, and the close — is written to the
// cash_movements ledger so the whole day is auditable and rolls into finance.
//
// Expected cash at any moment = opening float + cash sales collected + pay-ins
// and credit collected − withdrawals/refunds. The close compares this to the
// counted drawer and records the over/short difference.
package cashregister

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Withdraw records cash taken out of the drawer mid-shift (e.g. banked, paid to
// a supplier). Stored as a negative movement so it lowers expected cash.
// untrackedCashOutAllowed reports whether a till may move cash OUT of the drawer
// without naming a locker (the money leg then goes to External — untracked). Reads
// the shop policy directly; fails OPEN on a read error so a transient settings
// hiccup can never block a money move. Cash-IN (open / deposit) never consults it.
func (s *Service) untrackedCashOutAllowed(ctx context.Context) bool {
	var allow bool
	if err := s.db.GetContext(ctx, &allow, `SELECT allow_untracked_cash FROM settings WHERE id = 1`); err != nil {
		return true
	}
	return allow
}

func (s *Service) Withdraw(ctx context.Context, userID int64, in MovementInput) (*Summary, error) {
	if in.CounterLockerID == 0 && !s.untrackedCashOutAllowed(ctx) {
		return nil, apperr.Validation("untracked withdrawals are disabled — choose a locker to move the cash into")
	}
	return s.adjust(ctx, userID, in, MoveWithdrawal, true, "withdrawal")
}

// PayIn records cash added to the drawer mid-shift (e.g. extra float).
func (s *Service) PayIn(ctx context.Context, userID int64, in MovementInput) (*Summary, error) {
	return s.adjust(ctx, userID, in, MovePayIn, false, "pay-in")
}

func (s *Service) adjust(ctx context.Context, userID int64, in MovementInput, mtype string, negate bool, label string) (*Summary, error) {
	amt, err := money.Parse(in.Amount)
	if err != nil || !amt.IsPositive() {
		return nil, apperr.Validation(label + " amount must be positive")
	}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.Conflict("open a register session first")
		}
		return nil, apperr.Internal("failed to load session", err)
	}
	// A withdrawal can never exceed the cash currently in the drawer (opening +
	// cash sales + net adjustments — the same balance the close reconciles to).
	if mtype == MoveWithdrawal {
		cashSales, cerr := s.sales.CashCollectedSince(ctx, userID, sess.OpenedAt)
		if cerr != nil {
			return nil, cerr
		}
		adj, aerr := s.repo.AdjustmentTotal(ctx, sess.ID)
		if aerr != nil {
			return nil, apperr.Internal("failed to total cash movements", aerr)
		}
		available := sess.OpeningCash.Add(cashSales).Add(adj)
		if amt.GreaterThan(available) {
			return nil, apperr.Validation("cannot withdraw more than the " + money.Display(available) + " currently in the drawer")
		}
	}
	signed := amt
	if negate {
		signed = amt.Neg()
	}
	// The drawer movement and its money leg (locker side when a counterparty locker
	// is chosen, plus the receipt) commit together so the till and locker can never
	// drift.
	var receiptID int64
	if err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if merr := NewRepository(tx).AddMovement(ctx, sess.ID, userID, mtype, signed, in.Reason, nil); merr != nil {
			return keepAppError(merr, "failed to record cash movement")
		}
		if s.lockerLeg != nil {
			kind := "withdraw"
			if mtype == MovePayIn {
				kind = "payin"
			}
			rid, lerr := s.lockerLeg(ctx, tx, TillCashEvent{
				Kind: kind, UserID: userID, SessionID: sess.ID,
				LockerID: in.CounterLockerID, Amount: amt, Reason: in.Reason,
			})
			receiptID = rid
			return lerr
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if mtype == MoveWithdrawal {
		s.recordAudit(ctx, userID, audit.ActionWithdraw,
			"withdrew "+money.Display(amt)+" — "+in.Reason)
	}
	sum, err := s.Summary(ctx, userID)
	if err != nil {
		return nil, err
	}
	if receiptID > 0 {
		sum.ReceiptID = &receiptID
	}
	// Skip the standalone pulse when the till will fold it into the receipt print
	// (one job — see MovementInput.DeferDrawerKick). The drawer still pops there.
	// But an untracked move (no locker leg) has no receipt to merge into, so fall
	// back to kicking here — otherwise the drawer would never open.
	if !in.DeferDrawerKick || receiptID == 0 {
		s.kick(ctx)
	}
	return sum, nil
}

// RecordCreditCash logs cash collected against a customer's credit into the
// cashier's open drawer. It is a no-op when the cashier has no open session, so
// callers (the credit-collection page) don't need to special-case the drawer.
func (s *Service) RecordCreditCash(ctx context.Context, userID int64, amount decimal.Decimal, reason string) {
	if !amount.IsPositive() {
		return
	}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		return
	}
	_ = s.repo.AddMovement(ctx, sess.ID, userID, MoveCreditPayment, amount, reason, nil)
}

// RecordRefundCash logs a cash refund paid out of the cashier's open drawer for
// a return (negative, lowering expected cash). Like the other Record* helpers it
// is a no-op when no session is open or the amount isn't positive, so the return
// handler doesn't need to special-case the drawer.
func (s *Service) RecordRefundCash(ctx context.Context, userID int64, amount decimal.Decimal, reason string) {
	if !amount.IsPositive() {
		return
	}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		return
	}
	_ = s.repo.AddMovement(ctx, sess.ID, userID, MoveRefund, amount.Neg(), reason, nil)
}

// RecordSupplierCash logs cash paid out to a supplier from the cashier's open
// drawer as a withdrawal (negative, lowering expected cash). Like
// RecordCreditCash it is a no-op when no session is open, so the supplier-pay
// page doesn't need to special-case the drawer.
func (s *Service) RecordSupplierCash(ctx context.Context, userID int64, amount decimal.Decimal, reason string) {
	if !amount.IsPositive() {
		return
	}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		return
	}
	_ = s.repo.AddMovement(ctx, sess.ID, userID, MoveWithdrawal, amount.Neg(), reason, nil)
}
