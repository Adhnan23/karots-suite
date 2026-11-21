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
)

// Withdraw records cash taken out of the drawer mid-shift (e.g. banked, paid to
// a supplier). Stored as a negative movement so it lowers expected cash.
// untrackedCashOutAllowed reports whether a till may move cash OUT of the drawer
// without naming a locker (the money leg then goes to External — untracked). A
// setting that can't be read counts as off: cash never leaves untracked by
// accident (DRAW-12). Cash-IN (open / deposit) never consults it.
func (s *Service) untrackedCashOutAllowed(ctx context.Context) bool {
	var allow bool
	if err := s.db.GetContext(ctx, &allow, `SELECT allow_untracked_cash FROM settings WHERE id = 1`); err != nil {
		return false
	}
	return allow
}

func (s *Service) Withdraw(ctx context.Context, userID int64, in MovementInput) (int64, error) {
	if in.CounterLockerID == 0 && !s.untrackedCashOutAllowed(ctx) {
		return 0, apperr.Validation("untracked withdrawals are disabled — choose a locker to move the cash into")
	}
	return s.adjust(ctx, userID, in, MoveWithdrawal, true, "withdrawal")
}

// PayIn records cash added to the drawer mid-shift (e.g. extra float).
func (s *Service) PayIn(ctx context.Context, userID int64, in MovementInput) (int64, error) {
	return s.adjust(ctx, userID, in, MovePayIn, false, "pay-in")
}

// adjust returns the move's receipt (0 when none). Nothing after the commit can
// fail it: a saved move reported as an error would be done twice (DRAW-18).
func (s *Service) adjust(ctx context.Context, userID int64, in MovementInput, mtype string, negate bool, label string) (int64, error) {
	amt, err := money.Parse(in.Amount)
	if err != nil || !amt.IsPositive() {
		return 0, apperr.Validation(label + " amount must be positive")
	}
	signed := amt
	if negate {
		signed = amt.Neg()
	}
	// The session is locked first, and what the drawer holds is checked inside the
	// same transaction, so two withdrawals at once can't both pass (DRAW-16). The
	// drawer movement and its money leg (locker side when a counterparty locker is
	// chosen, plus the receipt) commit together so the till and locker can never drift.
	var receiptID int64
	if err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		sess, err := repo.FindOpenForUpdate(ctx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.Conflict("open a register session first")
		}
		if err != nil {
			return apperr.Internal("failed to load session", err)
		}
		if mtype == MoveWithdrawal { // never more than the cash in the drawer (DRAW-3)
			cashSales, err := s.sales.CashCollectedIn(ctx, sess.ID)
			if err != nil {
				return err
			}
			adj, err := repo.AdjustmentTotal(ctx, sess.ID)
			if err != nil {
				return apperr.Internal("failed to total cash movements", err)
			}
			if available := sess.OpeningCash.Add(cashSales).Add(adj); amt.GreaterThan(available) {
				return apperr.Validation("cannot withdraw more than the " + money.Display(available) + " currently in the drawer")
			}
		}
		if err := repo.AddMovement(ctx, sess.ID, userID, mtype, signed, in.Reason, nil); err != nil {
			return keepAppError(err, "failed to record cash movement")
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
		return 0, err
	}
	if mtype == MoveWithdrawal {
		s.recordAudit(ctx, userID, audit.ActionWithdraw,
			"withdrew "+money.Display(amt)+" — "+in.Reason)
	}
	// Skip the standalone pulse when the till will fold it into the receipt print
	// (one job — see MovementInput.DeferDrawerKick). The drawer still pops there.
	// But an untracked move (no locker leg) has no receipt to merge into, so fall
	// back to kicking here — otherwise the drawer would never open.
	if !in.DeferDrawerKick || receiptID == 0 {
		s.kick(ctx)
	}
	return receiptID, nil
}
