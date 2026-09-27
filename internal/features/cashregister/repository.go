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
	"strings"
	"time"

	appdb "karots-suite/internal/db"

	"github.com/shopspring/decimal"
)

type Repository struct{ q appdb.Queryer }

func NewRepository(q appdb.Queryer) *Repository { return &Repository{q: q} }

func (r *Repository) FindOpen(ctx context.Context, userID int64) (*Session, error) {
	var s Session
	err := r.q.GetContext(ctx, &s,
		`SELECT * FROM cash_register WHERE user_id = $1 AND closed_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindOpenForUpdate is FindOpen with a row lock, for use inside a transaction
// that moves cash to/from this drawer (cashflow.Move). The lock serialises
// concurrent withdrawals so two can't both pass the overdraw guard.
func (r *Repository) FindOpenForUpdate(ctx context.Context, userID int64) (*Session, error) {
	var s Session
	err := r.q.GetContext(ctx, &s,
		`SELECT * FROM cash_register WHERE user_id = $1 AND closed_at IS NULL FOR UPDATE`, userID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// LastClosed returns the cashier's most recently closed session (for the
// "continue with last amount" option when reopening).
func (r *Repository) LastClosed(ctx context.Context, userID int64) (*Session, error) {
	var s Session
	err := r.q.GetContext(ctx, &s,
		`SELECT * FROM cash_register WHERE user_id = $1 AND closed_at IS NOT NULL
		 ORDER BY closed_at DESC LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) Open(ctx context.Context, userID int64, opening decimal.Decimal, breakdown []byte) (*Session, error) {
	var s Session
	err := r.q.GetContext(ctx, &s,
		`INSERT INTO cash_register (user_id, opening_cash, opening_breakdown)
		 VALUES ($1, $2, $3) RETURNING *`,
		userID, opening, nullJSON(breakdown))
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) Close(ctx context.Context, id int64, closing, expected, diff decimal.Decimal, breakdown []byte) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE cash_register
		SET closing_cash = $1, expected_cash = $2, difference = $3, closing_breakdown = $4, closed_at = NOW()
		WHERE id = $5`, closing, expected, diff, nullJSON(breakdown), id)
	return err
}

func (r *Repository) AddMovement(ctx context.Context, sessionID, userID int64, mtype string, amount decimal.Decimal, reason string, breakdown []byte) error {
	var rp *string
	if strings.TrimSpace(reason) != "" {
		rp = &reason
	}
	_, err := r.q.ExecContext(ctx,
		`INSERT INTO cash_movements (session_id, user_id, type, amount, reason, breakdown)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		sessionID, userID, mtype, amount, rp, nullJSON(breakdown))
	return err
}

func (r *Repository) ListMovements(ctx context.Context, sessionID int64) ([]Movement, error) {
	var rows []Movement
	err := r.q.SelectContext(ctx, &rows,
		`SELECT * FROM cash_movements WHERE session_id = $1 ORDER BY created_at`, sessionID)
	return rows, err
}

// AdjustmentTotal sums the in/out movements that change expected drawer cash
// (everything except the opening/closing/sale audit rows).
func (r *Repository) AdjustmentTotal(ctx context.Context, sessionID int64) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.q.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(amount),0) FROM cash_movements
		 WHERE session_id = $1 AND type IN ('withdrawal','pay_in','credit_payment','refund','expense')`, sessionID)
	return total, err
}

func (r *Repository) RecentSessions(ctx context.Context, limit int) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id
		ORDER BY cr.opened_at DESC LIMIT $1`, limit)
	return rows, err
}

// OpenSessions lists every currently-open till (closed_at IS NULL) with its
// cashier name — for pickers that move cash to/from a specific drawer.
func (r *Repository) OpenSessions(ctx context.Context) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE cr.closed_at IS NULL
		ORDER BY cr.opened_at`)
	return rows, err
}

// SessionsInRange lists sessions opened within [from,to) for the period report.
func (r *Repository) SessionsInRange(ctx context.Context, from, to time.Time) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE cr.opened_at >= $1 AND cr.opened_at < $2
		ORDER BY cr.opened_at DESC`, from, to)
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*SessionRow, error) {
	var s SessionRow
	err := r.q.GetContext(ctx, &s, `
		SELECT cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id WHERE cr.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
