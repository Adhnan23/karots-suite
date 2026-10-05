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
	"encoding/json"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Movement types (mirror the cash_movement_type enum).
const (
	MoveOpening       = "opening"
	MoveSale          = "sale"
	MoveCreditPayment = "credit_payment"
	MoveWithdrawal    = "withdrawal"
	MovePayIn         = "pay_in"
	MoveRefund        = "refund"
	MoveClosing       = "closing"
	// MoveExpense is a counter expense paid from the drawer. It is an outflow like
	// a withdrawal for all drawer/close math; it exists so the ledger can show it
	// as "Expense" instead of lumping it in with manual withdrawals.
	MoveExpense = "expense"
)

type Session struct {
	ID               int64            `db:"id"                json:"id"`
	UserID           int64            `db:"user_id"           json:"user_id"`
	OpeningCash      decimal.Decimal  `db:"opening_cash"      json:"opening_cash"`
	ClosingCash      *decimal.Decimal `db:"closing_cash"      json:"closing_cash,omitempty"`
	ExpectedCash     *decimal.Decimal `db:"expected_cash"     json:"expected_cash,omitempty"`
	Difference       *decimal.Decimal `db:"difference"        json:"difference,omitempty"`
	OpeningBreakdown []byte           `db:"opening_breakdown" json:"opening_breakdown,omitempty"`
	ClosingBreakdown []byte           `db:"closing_breakdown" json:"closing_breakdown,omitempty"`
	OpenedAt         time.Time        `db:"opened_at"         json:"opened_at"`
	ClosedAt         *time.Time       `db:"closed_at"         json:"closed_at,omitempty"`
	// ReceiptID is the CR- money receipt written for the opening float (transient;
	// not a column). Non-nil lets the terminal print/prompt for the slip.
	ReceiptID *int64 `db:"-" json:"receipt_id,omitempty"`
}

type Movement struct {
	ID        int64           `db:"id"         json:"id"`
	SessionID int64           `db:"session_id" json:"session_id"`
	UserID    int64           `db:"user_id"    json:"user_id"`
	Type      string          `db:"type"       json:"type"`
	Amount    decimal.Decimal `db:"amount"     json:"amount"`
	Reason    *string         `db:"reason"     json:"reason,omitempty"`
	Breakdown []byte          `db:"breakdown"  json:"-"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}

// SessionRow is a session joined with the cashier name, for admin listings.
type SessionRow struct {
	Session
	UserName string `db:"user_name" json:"user_name"`
}

type OpenInput struct {
	OpeningCash string          `json:"opening_cash" form:"opening_cash"`
	Breakdown   json.RawMessage `json:"breakdown"`
	// SourceLockerID, when > 0, takes the opening float FROM that locker (debits
	// it + writes a receipt) instead of treating the float as untracked capital.
	SourceLockerID int64 `json:"source_locker_id" form:"source_locker_id"`
}

type CloseInput struct {
	ClosingCash string          `json:"closing_cash" form:"closing_cash"`
	Breakdown   json.RawMessage `json:"breakdown"`
	// DestLockerID, when > 0, banks the counted closing cash INTO that locker
	// (credits it + writes a receipt).
	DestLockerID int64 `json:"dest_locker_id" form:"dest_locker_id"`
}

// MovementInput is a mid-shift withdrawal or pay-in.
type MovementInput struct {
	Amount string `json:"amount" form:"amount" validate:"required"`
	Reason string `json:"reason" form:"reason"`
	// CounterLockerID, when > 0, names the locker the cash moves to (withdraw) or
	// from (pay-in), so the move is tracked with a receipt instead of vanishing.
	CounterLockerID int64 `json:"counter_locker_id" form:"counter_locker_id"`
	// DeferDrawerKick tells the service NOT to fire the standalone drawer pulse,
	// because the till (auto-print mode) will fold it into the receipt print as a
	// single job — avoiding a second printer job and its inter-job gap. In prompt
	// mode the client leaves this false so the drawer still pops at the cash event.
	DeferDrawerKick bool `json:"defer_drawer_kick" form:"defer_drawer_kick"`
}

// TillCashEvent describes a till cash event whose locker side must be booked.
// Passed to a LockerLeg hook (injected by the web layer) so cashregister never
// imports cashflow (cashflow already imports cashregister).
type TillCashEvent struct {
	Kind      string // "open" | "close" | "withdraw" | "payin"
	UserID    int64
	SessionID int64
	LockerID  int64
	Amount    decimal.Decimal
	Reason    string
}

// LockerLeg books the money side (+ receipt) of a till cash event, inside the
// caller's transaction. When ev.LockerID > 0 it moves cash to/from that locker;
// when 0 the till leg stands alone (opening float / plain drawer pay-in or
// withdrawal) and only the receipt is written. It returns the id of the CR-
// money receipt it created so the caller can print it. Injected via
// WithLockerLeg; nil = money integration off (no receipt, returns 0).
type LockerLeg func(ctx context.Context, tx *sqlx.Tx, ev TillCashEvent) (int64, error)

// CloseResult is returned after closing so the UI can show the reconciliation.
type CloseResult struct {
	Session      Session         `json:"session"`
	CashSales    decimal.Decimal `json:"cash_sales"`
	ExpectedCash decimal.Decimal `json:"expected_cash"`
	Difference   decimal.Decimal `json:"difference"`
	// ReceiptID is the CR- money receipt written for the counted/banked close.
	ReceiptID *int64 `json:"receipt_id,omitempty"`
}

// Summary is the live state of the open drawer, for the cashier terminal.
type Summary struct {
	Session       *Session         `json:"session"`
	CashSales     decimal.Decimal  `json:"cash_sales"`
	PayIns        decimal.Decimal  `json:"pay_ins"`     // pay-ins + credit collected
	Withdrawals   decimal.Decimal  `json:"withdrawals"` // positive magnitude
	Expected      decimal.Decimal  `json:"expected"`
	Movements     []Movement       `json:"movements"`
	LastClosing   *decimal.Decimal `json:"last_closing,omitempty"`
	LastBreakdown []byte           `json:"last_breakdown,omitempty"`
	// ReceiptID is the CR- money receipt written for the just-recorded pay-in or
	// withdrawal (transient; only set on the Summary returned by that action).
	ReceiptID *int64 `json:"receipt_id,omitempty"`
}

type Service struct {
	db         *sqlx.DB
	repo       *Repository
	sales      *sales.Service
	audit      *audit.Service            // optional; nil = no audit recording
	lockerLeg  LockerLeg                 // optional; nil = locker integration off
	drawerKick func(ctx context.Context) // optional; fires post-commit on a till cash event
}

func NewService(db *sqlx.DB, salesSvc *sales.Service) *Service {
	return &Service{db: db, repo: NewRepository(db), sales: salesSvc}
}

// WithAudit attaches an audit recorder so drawer withdrawals and closes are
// logged. Returns the service for chaining.
func (s *Service) WithAudit(a *audit.Service) *Service {
	s.audit = a
	return s
}

// WithLockerLeg injects the hook that books the locker side of a till cash event
// (opening float from a locker, banking at close, mid-shift moves to/from a
// locker). The web layer supplies it (cashflow.TillLockerLeg) so this package
// never imports cashflow. Returns the service for chaining.
func (s *Service) WithLockerLeg(fn LockerLeg) *Service {
	s.lockerLeg = fn
	return s
}

// WithDrawerKick injects a best-effort action fired AFTER a till cash event
// commits (open, close, pay-in, withdrawal) — used to pop the physical cash
// drawer. nil = no drawer. Returns the service for chaining.
func (s *Service) WithDrawerKick(fn func(ctx context.Context)) *Service {
	s.drawerKick = fn
	return s
}

func (s *Service) kick(ctx context.Context) {
	if s.drawerKick != nil {
		s.drawerKick(ctx)
	}
}

func (s *Service) recordAudit(ctx context.Context, userID int64, action, detail string) {
	if s.audit != nil {
		s.audit.Record(ctx, userID, action, "cash", "", detail)
	}
}

// Current returns the cashier's open session, or nil if none is open.
func (s *Service) Current(ctx context.Context, userID int64) (*Session, error) {
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, apperr.Internal("failed to load session", err)
	}
	return sess, nil
}

// Summary returns the live drawer state (or, when no session is open, the last
// closing so the UI can offer "continue with last amount").
func (s *Service) Summary(ctx context.Context, userID int64) (*Summary, error) {
	out := &Summary{}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.Internal("failed to load session", err)
		}
		// No open session — surface the last close for the reopen options.
		if last, lerr := s.repo.LastClosed(ctx, userID); lerr == nil {
			out.LastClosing = last.ClosingCash
			out.LastBreakdown = last.ClosingBreakdown
		}
		return out, nil
	}
	out.Session = sess
	out.CashSales, err = s.sales.CashCollectedSince(ctx, userID, sess.OpenedAt)
	if err != nil {
		return nil, err
	}
	moves, err := s.repo.ListMovements(ctx, sess.ID)
	if err != nil {
		return nil, apperr.Internal("failed to load cash movements", err)
	}
	out.Movements = moves
	for _, m := range moves {
		switch m.Type {
		case MovePayIn, MoveCreditPayment:
			out.PayIns = out.PayIns.Add(m.Amount)
		case MoveWithdrawal, MoveRefund, MoveExpense:
			out.Withdrawals = out.Withdrawals.Add(m.Amount.Abs())
		}
	}
	out.Expected = sess.OpeningCash.Add(out.CashSales).Add(out.PayIns).Sub(out.Withdrawals)
	return out, nil
}

// keepAppError preserves an error that already classified itself.
//
// The transaction bodies below call into cashflow and lockers, which return
// precise failures ("locker not found"). Wrapping every one of those in a
// blanket Internal turned a wrong pick into a 500 and showed the cashier a
// generic "something went wrong" for a problem they could have fixed.
func keepAppError(err error, fallback string) error {
	if ae, ok := apperr.As(err); ok {
		return ae
	}
	return apperr.Internal(fallback, err)
}

func (s *Service) Open(ctx context.Context, userID int64, in OpenInput) (*Session, error) {
	opening, raw, ok := amountFrom(in.OpeningCash, in.Breakdown)
	if !ok || opening.IsNegative() {
		return nil, apperr.Validation("opening cash must be a non-negative amount")
	}
	var sess *Session
	var receiptID int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		var ierr error
		sess, ierr = repo.Open(ctx, userID, opening, raw)
		if ierr != nil {
			return ierr
		}
		if ierr = repo.AddMovement(ctx, sess.ID, userID, MoveOpening, opening, "opening cash", raw); ierr != nil {
			return ierr
		}
		// Book the money side + receipt. With a source locker chosen, the opening
		// float is debited from it; without one it's untracked capital and only the
		// receipt is written. The till side is the session's opening_cash either way.
		if s.lockerLeg != nil && opening.IsPositive() {
			rid, lerr := s.lockerLeg(ctx, tx, TillCashEvent{
				Kind: "open", UserID: userID, SessionID: sess.ID,
				LockerID: in.SourceLockerID, Amount: opening, Reason: "opening cash",
			})
			receiptID = rid
			return lerr
		}
		return nil
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "idx_cash_register_open_session") {
			return nil, apperr.Conflict("you already have an open register session")
		}
		return nil, keepAppError(err, "failed to open register")
	}
	if receiptID > 0 {
		sess.ReceiptID = &receiptID
	}
	s.kick(ctx)
	return sess, nil
}

func (s *Service) Close(ctx context.Context, userID int64, in CloseInput) (*CloseResult, error) {
	closing, raw, ok := amountFrom(in.ClosingCash, in.Breakdown)
	if !ok || closing.IsNegative() {
		return nil, apperr.Validation("closing cash must be a non-negative amount")
	}
	if in.DestLockerID == 0 && closing.IsPositive() && !s.untrackedCashOutAllowed(ctx) {
		return nil, apperr.Validation("banking to a locker is required — choose a locker to close the cash into")
	}
	sess, err := s.repo.FindOpen(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.Conflict("no open register session to close")
		}
		return nil, apperr.Internal("failed to load session", err)
	}
	cashSales, err := s.sales.CashCollectedSince(ctx, userID, sess.OpenedAt)
	if err != nil {
		return nil, err
	}
	adj, err := s.repo.AdjustmentTotal(ctx, sess.ID)
	if err != nil {
		return nil, apperr.Internal("failed to total cash movements", err)
	}
	expected := sess.OpeningCash.Add(cashSales).Add(adj)
	diff := closing.Sub(expected)
	var receiptID int64
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		if cerr := repo.Close(ctx, sess.ID, closing, expected, diff, raw); cerr != nil {
			return cerr
		}
		if cerr := repo.AddMovement(ctx, sess.ID, userID, MoveClosing, closing, "counted at close", raw); cerr != nil {
			return cerr
		}
		// Book the money side + receipt. With a destination locker chosen the counted
		// cash is banked into it; without one only the receipt is written.
		if s.lockerLeg != nil && closing.IsPositive() {
			reason := "counted at close"
			if in.DestLockerID > 0 {
				reason = "banked at close"
			}
			rid, lerr := s.lockerLeg(ctx, tx, TillCashEvent{
				Kind: "close", UserID: userID, SessionID: sess.ID,
				LockerID: in.DestLockerID, Amount: closing, Reason: reason,
			})
			receiptID = rid
			return lerr
		}
		return nil
	})
	if err != nil {
		return nil, keepAppError(err, "failed to close register")
	}
	sess.ClosingCash = &closing
	sess.ExpectedCash = &expected
	sess.Difference = &diff
	s.recordAudit(ctx, userID, audit.ActionClose,
		"closed register: counted "+money.Display(closing)+", expected "+money.Display(expected)+", over/short "+money.Display(diff))
	res := &CloseResult{Session: *sess, CashSales: cashSales, ExpectedCash: expected, Difference: diff}
	if receiptID > 0 {
		res.ReceiptID = &receiptID
	}
	s.kick(ctx)
	return res, nil
}

// ListSessions pages the admin's till sessions (karots-pos showed the newest 100).
func (s *Service) ListSessions(ctx context.Context, f SessionFilter) ([]SessionRow, error) {
	rows, err := s.repo.ListSessions(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

func (s *Service) Cashiers(ctx context.Context) ([]SessionRow, error) {
	rows, err := s.repo.Cashiers(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list cashiers", err)
	}
	return rows, nil
}

func (s *Service) RecentSessions(ctx context.Context, limit int) ([]SessionRow, error) {
	rows, err := s.repo.RecentSessions(ctx, limit)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

// OpenSessions lists currently-open tills (for moving cash to/from a drawer).
func (s *Service) OpenSessions(ctx context.Context) ([]SessionRow, error) {
	rows, err := s.repo.OpenSessions(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list open tills", err)
	}
	return rows, nil
}

func (s *Service) SessionsInRange(ctx context.Context, from, to time.Time) ([]SessionRow, error) {
	rows, err := s.repo.SessionsInRange(ctx, from, to)
	if err != nil {
		return nil, apperr.Internal("failed to list register sessions", err)
	}
	return rows, nil
}

func (s *Service) SessionDetail(ctx context.Context, id int64) (*SessionRow, []Movement, error) {
	sess, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, apperr.NotFound("register session")
		}
		return nil, nil, apperr.Internal("failed to load session", err)
	}
	moves, err := s.repo.ListMovements(ctx, id)
	if err != nil {
		return nil, nil, apperr.Internal("failed to load cash movements", err)
	}
	return sess, moves, nil
}
