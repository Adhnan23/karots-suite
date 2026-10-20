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

	"karots-suite/internal/apperr"
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

// Close records the count on a still-open session; one already closed is a conflict
// (never closed twice). closedBy is nil when its own cashier closes it.
func (r *Repository) Close(ctx context.Context, id int64, closing, expected, diff decimal.Decimal, breakdown []byte, closedBy *int64) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE cash_register
		SET closing_cash = $1, expected_cash = $2, difference = $3, closing_breakdown = $4, closed_at = NOW(), closed_by = $6
		WHERE id = $5 AND closed_at IS NULL`, closing, expected, diff, nullJSON(breakdown), id, closedBy)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Conflict("that till is already closed")
	}
	return nil
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

// SessionFilter narrows the admin's session list: opened in [From, To), one
// cashier (0 = all), still open only. Newest first.
type SessionFilter struct {
	From, To *time.Time
	UserID   int64
	OpenOnly bool
	Limit    int
	Offset   int
}

func (r *Repository) ListSessions(ctx context.Context, f SessionFilter) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE ($1::timestamptz IS NULL OR cr.opened_at >= $1)
		  AND ($2::timestamptz IS NULL OR cr.opened_at <  $2)
		  AND ($3 = 0 OR cr.user_id = $3)
		  AND (NOT $4 OR cr.closed_at IS NULL)
		ORDER BY cr.opened_at DESC, cr.id DESC LIMIT NULLIF($5, 0) OFFSET $6`, f.From, f.To, f.UserID, f.OpenOnly, f.Limit, max(f.Offset, 0))
	return rows, err
}

// SessionTotals is every session the filter matches together (its Limit and
// Offset are ignored): count, money in and out of the drawers, over / short.
type SessionTotals struct {
	Sessions  int             `db:"sessions"`
	Opening   decimal.Decimal `db:"opening"`
	Expected  decimal.Decimal `db:"expected"`
	Counted   decimal.Decimal `db:"counted"`
	OverShort decimal.Decimal `db:"over_short"`
	Short     int             `db:"short"`
}

func (r *Repository) SessionTotals(ctx context.Context, f SessionFilter) (*SessionTotals, error) {
	var t SessionTotals
	err := r.q.GetContext(ctx, &t, `
		SELECT COUNT(*) AS sessions, COALESCE(SUM(opening_cash),0) AS opening,
		       COALESCE(SUM(expected_cash),0) AS expected, COALESCE(SUM(closing_cash),0) AS counted,
		       COALESCE(SUM(difference),0) AS over_short, COUNT(*) FILTER (WHERE difference < 0) AS short
		FROM cash_register cr
		WHERE ($1::timestamptz IS NULL OR cr.opened_at >= $1)
		  AND ($2::timestamptz IS NULL OR cr.opened_at <  $2)
		  AND ($3 = 0 OR cr.user_id = $3)
		  AND (NOT $4 OR cr.closed_at IS NULL)`, f.From, f.To, f.UserID, f.OpenOnly)
	return &t, err
}

// Cashiers lists everyone who has ever opened a till, for the list's filter.
func (r *Repository) Cashiers(ctx context.Context) ([]SessionRow, error) {
	var rows []SessionRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT DISTINCT ON (u.id) cr.*, u.name AS user_name
		FROM cash_register cr JOIN users u ON u.id = cr.user_id
		ORDER BY u.id, cr.opened_at DESC`)
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
