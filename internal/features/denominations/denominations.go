// Package denominations manages the notes and coins the shop handles, so the
// cashier can count the drawer by piece count (how many of each) instead of
// typing a total. Admins add/remove/edit denominations as currency changes.
package denominations

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type Denomination struct {
	ID        int64           `db:"id"         json:"id"`
	Value     decimal.Decimal `db:"value"      json:"value"`
	IsNote    bool            `db:"is_note"    json:"is_note"`
	IsActive  bool            `db:"is_active"  json:"is_active"`
	CreatedAt time.Time       `db:"created_at" json:"-"`
}

type Input struct {
	Value    string `json:"value"     form:"value"     validate:"required"`
	IsNote   bool   `json:"is_note"   form:"is_note"`
	IsActive bool   `json:"is_active" form:"is_active"`
}

type Repository struct{ db db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{db: q} }

// List returns denominations; activeOnly restricts to those in circulation
// (used by the cashier counting grid). Ordered high → low for natural counting.
func (r *Repository) List(ctx context.Context, activeOnly bool) ([]Denomination, error) {
	var rows []Denomination
	q := `SELECT * FROM denominations`
	if activeOnly {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY value DESC`
	err := r.db.SelectContext(ctx, &rows, q)
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Denomination, error) {
	var d Denomination
	err := r.db.GetContext(ctx, &d, `SELECT * FROM denominations WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) Create(ctx context.Context, value decimal.Decimal, isNote, isActive bool) (*Denomination, error) {
	var d Denomination
	err := r.db.GetContext(ctx, &d,
		`INSERT INTO denominations (value, is_note, is_active) VALUES ($1, $2, $3) RETURNING *`,
		value, isNote, isActive)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) Update(ctx context.Context, id int64, value decimal.Decimal, isNote, isActive bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE denominations SET value = $1, is_note = $2, is_active = $3 WHERE id = $4`,
		value, isNote, isActive, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM denominations WHERE id = $1`, id)
	return err
}

type Service struct{ repo *Repository }

func NewService(q db.Queryer) *Service { return &Service{repo: NewRepository(q)} }

func (s *Service) List(ctx context.Context, activeOnly bool) ([]Denomination, error) {
	rows, err := s.repo.List(ctx, activeOnly)
	if err != nil {
		return nil, apperr.Internal("failed to list denominations", err)
	}
	return rows, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Denomination, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("denomination")
		}
		return nil, apperr.Internal("failed to load denomination", err)
	}
	return d, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*Denomination, error) {
	value, err := decimal.NewFromString(in.Value)
	if err != nil || !value.IsPositive() {
		return nil, apperr.Validation("value must be a positive amount")
	}
	d, err := s.repo.Create(ctx, value, in.IsNote, in.IsActive)
	if err != nil {
		return nil, apperr.Conflict("a denomination with that value already exists")
	}
	return d, nil
}

func (s *Service) Update(ctx context.Context, id int64, in Input) error {
	value, err := decimal.NewFromString(in.Value)
	if err != nil || !value.IsPositive() {
		return apperr.Validation("value must be a positive amount")
	}
	err = s.repo.Update(ctx, id, value, in.IsNote, in.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("denomination")
	}
	if err != nil {
		return apperr.Conflict("a denomination with that value already exists")
	}
	return nil
}

// Delete removes a denomination no saved drawer count uses; one that a count
// does is refused (take it out of circulation instead), so old counts keep
// meaning what they said.
func (s *Service) Delete(ctx context.Context, id int64) error {
	d, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	var used bool
	if err := s.repo.db.GetContext(ctx, &used, `
		SELECT EXISTS (SELECT 1 FROM cash_register WHERE opening_breakdown ?| $1 OR closing_breakdown ?| $1)
		    OR EXISTS (SELECT 1 FROM cash_movements WHERE breakdown ?| $1)`,
		pq.Array([]string{d.Value.String(), d.Value.StringFixed(2)})); err != nil {
		return apperr.Internal("failed to check denomination use", err)
	}
	if used {
		return apperr.Conflict("a saved drawer count uses it — take it out of circulation instead")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperr.Internal("failed to delete denomination", err)
	}
	return nil
}
