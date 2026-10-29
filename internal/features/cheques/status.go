package cheques

import (
	"context"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/lockers"

	"github.com/jmoiron/sqlx"
)

const ymd = "2006-01-02"

// Clear marks a pending cheque cleared on day on (cheque date ≤ on ≤ today). A
// received cheque moves from Cheques in hand into the bank place intoLockerID; a
// given one leaves the bank place it was drawn on (intoLockerID is ignored).
func (s *Service) Clear(ctx context.Context, id int64, on time.Time, intoLockerID int64, actor int64) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error { return s.clearTx(ctx, tx, id, on, intoLockerID, actor) })
}

func (s *Service) clearTx(ctx context.Context, tx *sqlx.Tx, id int64, on time.Time, into int64, actor int64) error {
	c, err := lockPending(ctx, tx, id)
	if err != nil {
		return err
	}
	day := on.Format(ymd)
	if day < c.ChequeDate.Format(ymd) {
		return apperr.Validation("a cheque can't clear before its date")
	}
	if day > time.Now().Format(ymd) {
		return apperr.Validation("a cheque can't clear in the future")
	}
	ref := &cashflow.Ref{Kind: "cheque", ID: c.ID}
	reason := "cheque " + c.Number
	var detail string
	if c.Direction == "in" {
		name, err := activeBank(ctx, tx, into)
		if err != nil {
			return err
		}
		inHand, err := lockers.ChequesInHandTx(ctx, tx)
		if err != nil {
			return apperr.Internal("failed to find Cheques in hand", err)
		}
		if _, err := s.cf.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Locker(inHand), To: cashflow.Locker(into),
			Amount: c.Amount, Reason: reason, ReceiptKind: "transfer", Ref: ref, ActorID: actor}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE cheques SET locker_id = $1 WHERE id = $2`, into, c.ID); err != nil {
			return apperr.Internal("failed to update cheque", err)
		}
		detail = "cleared into " + name
	} else {
		kind, party := "expense", ""
		if c.SupplierID != nil {
			kind, party = "supplier_payment", deref(c.SupplierName)
		}
		switch {
		case c.SupplierPaymentID != nil:
			ref = &cashflow.Ref{Kind: "supplier_payment", ID: *c.SupplierPaymentID}
		case c.ExpenseID != nil:
			ref = &cashflow.Ref{Kind: "expense", ID: *c.ExpenseID}
		}
		// Recorded: the bank has already paid it — never refuse an overdraw here.
		if _, err := s.cf.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Locker(*c.LockerID), To: cashflow.External(),
			Amount: c.Amount, Reason: reason, ReceiptKind: kind, Ref: ref, Party: party, ActorID: actor, Recorded: true}); err != nil {
			return err
		}
		detail = "cleared from " + deref(c.LockerName)
	}
	return setStatus(ctx, tx, c.ID, "cleared", day, actor, detail)
}

// Return marks a pending cheque bounced or cancelled (the same for the money):
// see undo for what it puts back.
func (s *Service) Return(ctx context.Context, id int64, status string, actor int64) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error { return s.returnTx(ctx, tx, id, status, actor) })
}

func (s *Service) returnTx(ctx context.Context, tx *sqlx.Tx, id int64, status string, actor int64) error {
	if status != "bounced" && status != "cancelled" {
		return apperr.Validation("status must be bounced or cancelled")
	}
	c, err := lockPending(ctx, tx, id)
	if err != nil {
		return err
	}
	if err := s.undo(ctx, tx, c, status, actor); err != nil {
		return err
	}
	return setStatus(ctx, tx, c.ID, status, time.Now().Format(ymd), actor, status)
}

// FixInput corrects a pending cheque's details (never its amount or person).
type FixInput struct {
	Bank     string
	LockerID int64
	Number   string
	Date     string // YYYY-MM-DD
	Note     string
}

func (s *Service) Fix(ctx context.Context, id int64, in FixInput, actor int64) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error { return fixTx(ctx, tx, id, in, actor) })
}

func fixTx(ctx context.Context, tx *sqlx.Tx, id int64, in FixInput, actor int64) error {
	c, err := lockPending(ctx, tx, id)
	if err != nil {
		return err
	}
	bank, number, note := strings.TrimSpace(in.Bank), strings.TrimSpace(in.Number), strings.TrimSpace(in.Note)
	if number == "" {
		return apperr.Validation("enter the cheque number")
	}
	d, err := time.Parse(ymd, strings.TrimSpace(in.Date))
	if err != nil {
		return apperr.Validation("enter the cheque date")
	}
	locker := c.LockerID
	if c.Direction == "in" {
		if bank == "" {
			return apperr.Validation("pick the cheque's bank")
		}
	} else {
		if _, err := activeBank(ctx, tx, in.LockerID); err != nil {
			return err
		}
		locker, bank = &in.LockerID, ""
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cheques SET bank = $1, locker_id = $2, number = $3, cheque_date = $4, note = $5 WHERE id = $6`,
		bank, locker, number, d.Format(ymd), note, c.ID); err != nil {
		return apperr.Internal("failed to update cheque", err)
	}
	return auditTx(ctx, tx, actor, c.ID, "details fixed: "+strings.TrimSpace(bank+" "+number+" "+d.Format(ymd)))
}

func setStatus(ctx context.Context, tx *sqlx.Tx, id int64, status, day string, actor int64, detail string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE cheques SET status = $1, status_date = $2, status_by = $3 WHERE id = $4`,
		status, day, nz(actor), id); err != nil {
		return apperr.Internal("failed to update cheque", err)
	}
	return auditTx(ctx, tx, actor, id, detail)
}

func auditTx(ctx context.Context, tx *sqlx.Tx, actor, id int64, detail string) error {
	if err := audit.NewRepository(tx).Insert(ctx, actor, audit.ActionUpdate, "cheque", idStr(id), detail); err != nil {
		return apperr.Internal("failed to record cheque history", err)
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
