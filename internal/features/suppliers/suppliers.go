// Package suppliers manages wholesale suppliers and their outstanding payables.
package suppliers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Supplier struct {
	ID                 int64           `db:"id"                  json:"id"`
	Name               string          `db:"name"                json:"name"`
	ContactPerson      *string         `db:"contact_person"      json:"contact_person,omitempty"`
	Phone              *string         `db:"phone"               json:"phone,omitempty"`
	Address            *string         `db:"address"             json:"address,omitempty"`
	CreditDays         int             `db:"credit_days"         json:"credit_days"`
	OutstandingBalance decimal.Decimal `db:"outstanding_balance" json:"outstanding_balance"`
	OpeningBalance     decimal.Decimal `db:"opening_balance"     json:"opening_balance"`
	OpeningUnlinked    decimal.Decimal `db:"opening_unlinked"    json:"opening_unlinked"`
	IsActive           bool            `db:"is_active"           json:"is_active"`
	CreatedAt          time.Time       `db:"created_at"          json:"created_at"`
}

// LinkedBalance is the transactional (document-backed) part of what we owe this
// supplier: the outstanding payable minus the still-unpaid old (opening) debt.
// This is read-only in the UI; only the opening part is editable.
func (s Supplier) LinkedBalance() decimal.Decimal {
	return s.OutstandingBalance.Sub(s.OpeningUnlinked)
}

type CreateInput struct {
	Name          string  `json:"name"           form:"name"           validate:"required,min=2,max=150"`
	ContactPerson *string `json:"contact_person" form:"contact_person"`
	Phone         *string `json:"phone"          form:"phone"          validate:"omitempty,max=15"`
	Address       *string `json:"address"        form:"address"`
	CreditDays    int     `json:"credit_days"    form:"credit_days"    validate:"gte=0"`
	// OpeningBalance is the amount we already owed this supplier at onboarding.
	// Applied once, at creation; Update never touches it.
	OpeningBalance string `json:"opening_balance" form:"opening_balance"`
}

type UpdateInput = CreateInput

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

func (s *Service) List(ctx context.Context, search string) ([]Supplier, error) {
	rows, err := s.repo.List(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list suppliers", err)
	}
	return rows, nil
}

// ListAll includes disabled suppliers (for the admin list with "Show disabled").
func (s *Service) ListAll(ctx context.Context, search string) ([]Supplier, error) {
	rows, err := s.repo.ListAll(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list suppliers", err)
	}
	return rows, nil
}

// Reactivate re-enables a disabled supplier.
func (s *Service) Reactivate(ctx context.Context, id int64) error {
	if err := s.repo.Reactivate(ctx, id); err != nil {
		return apperr.Internal("failed to reactivate supplier", err)
	}
	return nil
}

// Owing lists suppliers with an outstanding payable (for the dues report).
func (s *Service) Owing(ctx context.Context) ([]OwingRow, error) {
	rows, err := s.repo.Owing(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list supplier dues", err)
	}
	return rows, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Supplier, error) {
	sup, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("supplier")
		}
		return nil, apperr.Internal("failed to load supplier", err)
	}
	return sup, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Supplier, error) {
	in.Name = strings.TrimSpace(in.Name)
	// No two active suppliers should share a name (case-insensitive). Without this
	// a repeated "Add supplier" silently spawned a duplicate. The importer's own
	// upsert (ImportOne / FindOrCreateByName) is a separate, intentional path.
	if existing, ferr := s.repo.FindByName(ctx, in.Name); ferr == nil && existing != nil {
		return nil, apperr.Conflict("a supplier named \"" + existing.Name + "\" already exists")
	} else if ferr != nil && !errors.Is(ferr, sql.ErrNoRows) {
		return nil, apperr.Internal("failed to check for an existing supplier", ferr)
	}
	opening, err := parseOpening(in.OpeningBalance)
	if err != nil {
		return nil, err
	}
	sup, err := s.repo.Create(ctx, in, opening)
	if err != nil {
		return nil, apperr.Internal("failed to create supplier", err)
	}
	return sup, nil
}

// parseOpening parses an optional opening-balance string (blank → 0), rejecting
// negatives. We can't owe a supplier a negative amount at onboarding.
func parseOpening(s string) (decimal.Decimal, error) {
	if strings.TrimSpace(s) == "" {
		return decimal.Zero, nil
	}
	v, err := money.Parse(s)
	if err != nil || v.IsNegative() {
		return decimal.Zero, apperr.Validation("opening balance must be a non-negative amount")
	}
	return v, nil
}

// FindOrCreateByName resolves a supplier by case-insensitive name, creating a
// bare supplier (name only) if none exists. An empty name returns (nil, nil) so
// the caller can leave the link unset. Used by the bulk product importer.
func (s *Service) FindOrCreateByName(ctx context.Context, name string) (*int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if sup, err := s.repo.FindByName(ctx, name); err == nil {
		return &sup.ID, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Internal("failed to resolve supplier", err)
	}
	sup, err := s.repo.Create(ctx, CreateInput{Name: name}, decimal.Zero)
	if err != nil {
		return nil, apperr.Internal("failed to create supplier "+name, err)
	}
	return &sup.ID, nil
}

// ImportResult reports what one import row did, for the summary.
type ImportResult struct {
	Action string // "created" | "updated"
	Note   string
}

// ImportOne upserts one supplier in a transaction, matching an existing active
// supplier by case-insensitive name. The opening balance (already parsed into
// in.OpeningBalance) is applied on create only, so re-imports never re-add it.
// given names the columns the sheet fills ("contact_person", "phone",
// "address", "credit_days"); an existing supplier keeps the rest. nil = all.
func (s *Service) ImportOne(ctx context.Context, in CreateInput, opening decimal.Decimal, given map[string]bool) (ImportResult, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return ImportResult{}, apperr.Validation("name is required")
	}
	var res ImportResult
	err := db.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		r := NewRepository(tx)
		if existing, ferr := r.FindByName(ctx, in.Name); ferr == nil {
			if given != nil {
				if !given["contact_person"] {
					in.ContactPerson = existing.ContactPerson
				}
				if !given["phone"] {
					in.Phone = existing.Phone
				}
				if !given["address"] {
					in.Address = existing.Address
				}
				if !given["credit_days"] {
					in.CreditDays = existing.CreditDays
				}
			}
			if uerr := r.Update(ctx, existing.ID, in); uerr != nil {
				return uerr
			}
			res.Action = "updated"
			if opening.IsPositive() {
				res.Note = "opening balance skipped (existing supplier)"
			}
			return nil
		} else if !errors.Is(ferr, sql.ErrNoRows) {
			return ferr
		}
		if _, cerr := r.Create(ctx, in, opening); cerr != nil {
			return cerr
		}
		res.Action = "created"
		return nil
	})
	if err != nil {
		return ImportResult{}, apperr.Internal("failed to import supplier", err)
	}
	return res, nil
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	err := s.repo.Update(ctx, id, in)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("supplier")
	}
	if err != nil {
		return apperr.Internal("failed to update supplier", err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Deactivate(ctx, id); err != nil {
		return apperr.Internal("failed to remove supplier", err)
	}
	return nil
}

// AdjustOpening corrects the editable opening (old pre-system debt) for a
// supplier. It returns the supplier as it was before the change (for the audit
// trail) and the refreshed supplier. The transactional (linked) part is never
// touched — only the opening and, by the same delta, the outstanding payable.
func (s *Service) AdjustOpening(ctx context.Context, id int64, newOpeningStr string) (before, after *Supplier, err error) {
	// The opening (old pre-system balance) may be negative here: a supplier we were
	// in credit with before go-live (an advance/overpayment they still owe us).
	newOpening, perr := money.Parse(newOpeningStr)
	if perr != nil {
		return nil, nil, apperr.Validation("opening balance must be a valid amount")
	}
	before, err = s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if aerr := s.repo.AdjustOpening(ctx, id, newOpening); aerr != nil {
		if errors.Is(aerr, sql.ErrNoRows) {
			return nil, nil, apperr.NotFound("supplier")
		}
		return nil, nil, apperr.Internal("failed to adjust opening balance", aerr)
	}
	after, err = s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return before, after, nil
}
