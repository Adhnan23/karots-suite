// Package customers manages registered customers, especially credit ("vade")
// buyers. Walk-in sales leave customer_id NULL.
package customers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"karots-suite/internal/db"

	"github.com/shopspring/decimal"
)

type Repository struct{ q db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{q: q} }

func (r *Repository) List(ctx context.Context, search string) ([]Customer, error) {
	var rows []Customer
	var s *string
	if strings.TrimSpace(search) != "" {
		s = &search
	}
	err := r.q.SelectContext(ctx, &rows, `
		SELECT * FROM customers
		WHERE is_active = true
		  AND ($1::text IS NULL OR name ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')
		ORDER BY name LIMIT 100`, s)
	return rows, err
}

// AllActive returns every active customer (no limit), ordered by name — for the
// CSV export round-trip.
func (r *Repository) AllActive(ctx context.Context) ([]Customer, error) {
	var rows []Customer
	err := r.q.SelectContext(ctx, &rows, `SELECT * FROM customers WHERE is_active = true ORDER BY name`)
	return rows, err
}

// OwingRow is a customer with an outstanding balance, plus the date of their
// oldest unpaid credit sale (a proxy for aging — the system tracks an aggregate
// balance, not per-invoice allocation).
type OwingRow struct {
	Customer
	OldestCredit *time.Time `db:"oldest_credit" json:"oldest_credit,omitempty"`
}

// Owing lists active customers who currently owe money, biggest balance first.
func (r *Repository) Owing(ctx context.Context) ([]OwingRow, error) {
	var rows []OwingRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT c.*,
		       (SELECT MIN(s.created_at) FROM sales s
		         WHERE s.customer_id = c.id AND s.status = 'credit') AS oldest_credit
		FROM customers c
		WHERE c.is_active = true AND c.outstanding_balance > 0
		ORDER BY c.outstanding_balance DESC`)
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Customer, error) {
	var c Customer
	err := r.q.GetContext(ctx, &c, `SELECT * FROM customers WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindByPhone looks up an active customer by exact phone (the bulk-import upsert
// key). Returns sql.ErrNoRows when absent.
func (r *Repository) FindByPhone(ctx context.Context, phone string) (*Customer, error) {
	var c Customer
	err := r.q.GetContext(ctx, &c,
		`SELECT * FROM customers WHERE phone = $1 AND is_active = true ORDER BY id LIMIT 1`, phone)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) Create(ctx context.Context, name string, phone, address *string, limit, opening decimal.Decimal) (*Customer, error) {
	var c Customer
	err := r.q.GetContext(ctx, &c, `
		INSERT INTO customers (name, phone, address, credit_limit, opening_balance, outstanding_balance, opening_unlinked)
		VALUES ($1,$2,$3,$4,$5,$5,$5) RETURNING *`, name, phone, address, limit, opening)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// AddBalance increments outstanding balance (used inside the sale tx for the
// credit portion). Pass a negative delta for repayments. A repayment settles the
// transactional (linked) part first; any overflow erodes the still-unpaid opening
// debt, so opening_unlinked is clamped to never exceed the new outstanding balance.
func (r *Repository) AddBalance(ctx context.Context, id int64, delta decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE customers
		SET outstanding_balance = outstanding_balance + $1,
		    opening_unlinked = LEAST(opening_unlinked, GREATEST(outstanding_balance + $1, 0))
		WHERE id = $2`, delta, id)
	return err
}

// PayOpening reduces the old (opening) debt directly: outstanding_balance and
// opening_unlinked each drop by amt (clamped at zero), leaving the gross
// opening_balance and the linked (transactional) part untouched. Contrast
// AddBalance, which settles the linked part first. Callers must guard against
// overpaying the opening; the clamp here is a safety net, not the policy.
func (r *Repository) PayOpening(ctx context.Context, id int64, amt decimal.Decimal) error {
	// Reduce both columns by the SAME capped amount (never more than the old debt
	// still owed) so the linked part is preserved — clamping outstanding on its
	// own would wipe out linked on an overpay.
	_, err := r.q.ExecContext(ctx, `
		UPDATE customers SET
			outstanding_balance = outstanding_balance - GREATEST(LEAST($1, opening_unlinked), 0),
			opening_unlinked    = opening_unlinked    - GREATEST(LEAST($1, opening_unlinked), 0)
		WHERE id = $2`, amt, id)
	return err
}

// AdjustOpening sets the still-unpaid opening (old pre-system debt) to newOpening
// and shifts both the outstanding balance and the gross opening figure by the same
// delta, so the transactional (linked) part is left untouched. newOpening must be
// non-negative.
func (r *Repository) AdjustOpening(ctx context.Context, id int64, newOpening decimal.Decimal) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE customers SET
			outstanding_balance = outstanding_balance + ($1 - opening_unlinked),
			opening_balance     = opening_balance     + ($1 - opening_unlinked),
			opening_unlinked    = $1
		WHERE id = $2 AND is_active = true`, newOpening, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// InsertPayment logs a credit repayment and returns its id. createdBy of 0
// stores NULL.
func (r *Repository) InsertPayment(ctx context.Context, customerID int64, amount decimal.Decimal, method string, reference, note *string, createdBy int64) (int64, error) {
	var cb *int64
	if createdBy > 0 {
		cb = &createdBy
	}
	var id int64
	err := r.q.GetContext(ctx, &id,
		`INSERT INTO customer_payments (customer_id, amount, method, reference, note, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		customerID, amount, method, reference, note, cb)
	return id, err
}

// ListAll includes inactive customers (for the admin list, so they can be
// reactivated). Active first, then by name. Search matches name/phone.
func (r *Repository) ListAll(ctx context.Context, search string) ([]Customer, error) {
	var rows []Customer
	var s *string
	if strings.TrimSpace(search) != "" {
		s = &search
	}
	err := r.q.SelectContext(ctx, &rows, `
		SELECT * FROM customers
		WHERE ($1::text IS NULL OR name ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')
		ORDER BY is_active DESC, name LIMIT 100`, s)
	return rows, err
}

func (r *Repository) Deactivate(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE customers SET is_active=false WHERE id=$1`, id)
	return err
}

func (r *Repository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE customers SET is_active=true WHERE id=$1`, id)
	return err
}

// SetCreditLimit updates only a customer's credit limit (the till's inline
// raise). Unlike Update it never touches name/phone/address.
func (r *Repository) SetCreditLimit(ctx context.Context, id int64, limit decimal.Decimal) error {
	res, err := r.q.ExecContext(ctx,
		`UPDATE customers SET credit_limit = $1 WHERE id = $2 AND is_active = true`, limit, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, id int64, name string, phone, address *string, limit decimal.Decimal) error {
	res, err := r.q.ExecContext(ctx,
		`UPDATE customers SET name=$1, phone=$2, address=$3, credit_limit=$4 WHERE id=$5 AND is_active=true`,
		name, phone, address, limit, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
