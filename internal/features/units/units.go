// Package units manages units of measure (kg, pcs, ltr, ...).
package units

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
)

type Unit struct {
	ID           int64  `db:"id"            json:"id"`
	Name         string `db:"name"          json:"name"`
	Abbreviation string `db:"abbreviation"  json:"abbreviation"`
	AllowDecimal bool   `db:"allow_decimal" json:"allow_decimal"`
}

type Input struct {
	Name         string `json:"name"          form:"name"          validate:"required,min=1,max=30"`
	Abbreviation string `json:"abbreviation"  form:"abbreviation"  validate:"required,min=1,max=10"`
	AllowDecimal bool   `json:"allow_decimal" form:"allow_decimal"`
}

type Repository struct{ db db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{db: q} }

func (r *Repository) List(ctx context.Context) ([]Unit, error) {
	var rows []Unit
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM units ORDER BY name`)
	return rows, err
}

func (r *Repository) Create(ctx context.Context, name, abbr string, allowDecimal bool) (*Unit, error) {
	var u Unit
	err := r.db.GetContext(ctx, &u,
		`INSERT INTO units (name, abbreviation, allow_decimal) VALUES ($1, $2, $3) RETURNING *`, name, abbr, allowDecimal)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) Update(ctx context.Context, id int64, name, abbr string, allowDecimal bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE units SET name = $1, abbreviation = $2, allow_decimal = $3 WHERE id = $4`, name, abbr, allowDecimal, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM units WHERE id = $1`, id)
	return err
}

type Service struct{ repo *Repository }

func NewService(q db.Queryer) *Service { return &Service{repo: NewRepository(q)} }

func (s *Service) List(ctx context.Context) ([]Unit, error) {
	rows, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list units", err)
	}
	return rows, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*Unit, error) {
	u, err := s.repo.Create(ctx, in.Name, in.Abbreviation, in.AllowDecimal)
	if err != nil {
		return nil, apperr.Conflict("a unit with that name or abbreviation already exists")
	}
	return u, nil
}

func (s *Service) Update(ctx context.Context, id int64, in Input) error {
	err := s.repo.Update(ctx, id, in.Name, in.Abbreviation, in.AllowDecimal)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("unit")
	}
	if err != nil {
		return apperr.Conflict("a unit with that name or abbreviation already exists")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperr.Conflict("unit is in use and cannot be deleted")
	}
	return nil
}
