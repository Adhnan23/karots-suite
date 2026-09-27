// Package suppliers manages wholesale suppliers and their outstanding payables.
package suppliers

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

func (r *Repository) List(ctx context.Context, search string) ([]Supplier, error) {
	var rows []Supplier
	var s *string
	if strings.TrimSpace(search) != "" {
		s = &search
	}
	err := r.q.SelectContext(ctx, &rows, `
		SELECT * FROM suppliers
		WHERE is_active = true
		  AND ($1::text IS NULL OR name ILIKE '%' || $1 || '%'
		       OR contact_person ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')
		ORDER BY name`, s)
	return rows, err
}

// OwingRow is a supplier we owe money, plus the date of their oldest unpaid
// purchase (an aging proxy).
type OwingRow struct {
	Supplier
	OldestUnpaid *time.Time `db:"oldest_unpaid" json:"oldest_unpaid,omitempty"`
}

// Owing lists active suppliers with an outstanding payable, biggest first.
func (r *Repository) Owing(ctx context.Context) ([]OwingRow, error) {
	var rows []OwingRow
	err := r.q.SelectContext(ctx, &rows, `
		SELECT s.*,
		       (SELECT MIN(pu.created_at) FROM purchases pu
		         WHERE pu.supplier_id = s.id AND pu.status <> 'draft' AND pu.total > pu.paid_amount) AS oldest_unpaid
		FROM suppliers s
		WHERE s.is_active = true AND s.outstanding_balance > 0
		ORDER BY s.outstanding_balance DESC`)
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Supplier, error) {
	var s Supplier
	err := r.q.GetContext(ctx, &s, `SELECT * FROM suppliers WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByName looks up an active supplier by case-insensitive name. Returns
// sql.ErrNoRows when absent.
func (r *Repository) FindByName(ctx context.Context, name string) (*Supplier, error) {
	var s Supplier
	err := r.q.GetContext(ctx, &s,
		`SELECT * FROM suppliers WHERE lower(name) = lower($1) AND is_active = true ORDER BY id LIMIT 1`, name)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput, opening decimal.Decimal) (*Supplier, error) {
	var s Supplier
	err := r.q.GetContext(ctx, &s, `
		INSERT INTO suppliers (name, contact_person, phone, address, credit_days, opening_balance, outstanding_balance, opening_unlinked)
		VALUES ($1,$2,$3,$4,$5,$6,$6,$6) RETURNING *`,
		in.Name, in.ContactPerson, in.Phone, in.Address, in.CreditDays, opening)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) Update(ctx context.Context, id int64, in UpdateInput) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE suppliers SET name=$1, contact_person=$2, phone=$3, address=$4, credit_days=$5
		WHERE id=$6`, in.Name, in.ContactPerson, in.Phone, in.Address, in.CreditDays, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListAll returns active + disabled suppliers (disabled last), for the admin list
// with "Show disabled" on so they can be reactivated.
func (r *Repository) ListAll(ctx context.Context, search string) ([]Supplier, error) {
	var rows []Supplier
	var s *string
	if strings.TrimSpace(search) != "" {
		s = &search
	}
	err := r.q.SelectContext(ctx, &rows, `
		SELECT * FROM suppliers
		WHERE ($1::text IS NULL OR name ILIKE '%' || $1 || '%'
		       OR contact_person ILIKE '%' || $1 || '%' OR phone ILIKE '%' || $1 || '%')
		ORDER BY is_active DESC, name`, s)
	return rows, err
}

func (r *Repository) Deactivate(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE suppliers SET is_active=false WHERE id=$1`, id)
	return err
}

func (r *Repository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE suppliers SET is_active=true WHERE id=$1`, id)
	return err
}

// AddBalance changes a supplier's payable (used inside the purchase tx; pass a
// negative delta when paying the supplier). A payment settles the transactional
// (linked) part first; any overflow erodes the still-unpaid opening debt, so
// opening_unlinked is clamped to never exceed the new outstanding balance.
func (r *Repository) AddBalance(ctx context.Context, id int64, delta decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE suppliers
		SET outstanding_balance = outstanding_balance + $1,
		    opening_unlinked = LEAST(opening_unlinked, GREATEST(outstanding_balance + $1, 0))
		WHERE id=$2`, delta, id)
	return err
}

// PayOpening reduces the old (opening) debt directly: outstanding_balance and
// opening_unlinked each drop by amt (capped at the old debt still owed), leaving
// the gross opening_balance and the linked part untouched. Callers guard against
// overpaying; the clamp here is a safety net. Reducing both columns by the SAME
// capped amount preserves linked — clamping outstanding alone would wipe it out.
func (r *Repository) PayOpening(ctx context.Context, id int64, amt decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE suppliers SET
			outstanding_balance = outstanding_balance - GREATEST(LEAST($1, opening_unlinked), 0),
			opening_unlinked    = opening_unlinked    - GREATEST(LEAST($1, opening_unlinked), 0)
		WHERE id=$2`, amt, id)
	return err
}

// AdjustOpening sets the still-unpaid opening (old pre-system debt) to newOpening
// and shifts both the outstanding payable and the gross opening figure by the same
// delta, so the transactional (linked) part is left untouched. newOpening must be
// non-negative.
func (r *Repository) AdjustOpening(ctx context.Context, id int64, newOpening decimal.Decimal) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE suppliers SET
			outstanding_balance = outstanding_balance + ($1 - opening_unlinked),
			opening_balance     = opening_balance     + ($1 - opening_unlinked),
			opening_unlinked    = $1
		WHERE id=$2`, newOpening, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
