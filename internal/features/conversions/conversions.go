// Package conversions handles breaking one product into another — e.g. a "bag of
// rice" (1 unit) into "loose rice" (25 kg). Running a conversion depletes the
// source product's stock FEFO and opens a new batch of the destination product,
// preserving the inventory value.
package conversions

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Conversion struct {
	ID            int64           `db:"id"              json:"id"`
	FromProductID int64           `db:"from_product_id" json:"from_product_id"`
	ToProductID   int64           `db:"to_product_id"   json:"to_product_id"`
	Ratio         decimal.Decimal `db:"ratio"           json:"ratio"`
	Note          *string         `db:"note"            json:"note,omitempty"`
	IsActive      bool            `db:"is_active"       json:"is_active"`
	CreatedAt     time.Time       `db:"created_at"      json:"created_at"`
	// joined
	FromName     string `db:"from_name"      json:"from_name"`
	ToName       string `db:"to_name"        json:"to_name"`
	FromUnitAbbr string `db:"from_unit_abbr" json:"from_unit_abbr"`
	ToUnitAbbr   string `db:"to_unit_abbr"   json:"to_unit_abbr"`
	// On-hand quantities. Without these the Run dialog asks "how many?" while
	// hiding the only number that answers it.
	FromStock decimal.Decimal `db:"from_stock" json:"from_stock"`
	ToStock   decimal.Decimal `db:"to_stock"   json:"to_stock"`
	// FromCost is the source's current cost: the run sheet's "cost per kg
	// higher" hint when less comes out than the ratio says.
	FromCost decimal.Decimal `db:"from_cost" json:"from_cost"`
}

// Runnable reports whether there is any source stock to convert at all.
func (c Conversion) Runnable() bool { return c.FromStock.IsPositive() }

// MaxRuns is how many whole source units are available to convert.
func (c Conversion) MaxRuns() decimal.Decimal { return c.FromStock }

type CreateInput struct {
	FromProductID int64   `json:"from_product_id" form:"from_product_id" validate:"required,gt=0"`
	ToProductID   int64   `json:"to_product_id"   form:"to_product_id"   validate:"required,gt=0"`
	Ratio         string  `json:"ratio"           form:"ratio"           validate:"required"`
	Note          *string `json:"note"            form:"note"`
}

type UpdateInput struct {
	Ratio string  `json:"ratio" form:"ratio" validate:"required"`
	Note  *string `json:"note"  form:"note"`
}

type Repository struct{ q appdb.Queryer }

func NewRepository(q appdb.Queryer) *Repository { return &Repository{q: q} }

const selectConversion = `
	SELECT cv.*, fp.name AS from_name, tp.name AS to_name,
	       fu.abbreviation AS from_unit_abbr, tu.abbreviation AS to_unit_abbr,
	       COALESCE(fs.quantity, 0) AS from_stock,
	       COALESCE(ts.quantity, 0) AS to_stock,
	       fp.cost_price AS from_cost
	FROM product_conversions cv
	JOIN products fp ON fp.id = cv.from_product_id
	JOIN products tp ON tp.id = cv.to_product_id
	JOIN units fu ON fu.id = fp.unit_id
	JOIN units tu ON tu.id = tp.unit_id
	LEFT JOIN stock fs ON fs.product_id = cv.from_product_id
	LEFT JOIN stock ts ON ts.product_id = cv.to_product_id`

// List returns the active conversions, optionally filtered by a search over
// either product's name so a long recipe list stays navigable.
func (r *Repository) List(ctx context.Context, search string) ([]Conversion, error) {
	var rows []Conversion
	err := r.q.SelectContext(ctx, &rows, selectConversion+`
		WHERE cv.is_active = true
		  AND ($1::text IS NULL
		       OR fp.name ILIKE '%' || $1 || '%'
		       OR fp.name_local ILIKE '%' || $1 || '%'
		       OR tp.name ILIKE '%' || $1 || '%'
		       OR tp.name_local ILIKE '%' || $1 || '%')
		ORDER BY fp.name`, nullIfBlank(search))
	return rows, err
}

func nullIfBlank(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	t := strings.TrimSpace(s)
	return &t
}

// Update changes an existing recipe. Editing matters because a wrong ratio can
// only otherwise be fixed by deleting and recreating, which loses the link from
// past runs (conversion_runs.conversion_id is ON DELETE SET NULL).
func (r *Repository) Update(ctx context.Context, id int64, ratio decimal.Decimal, note *string) error {
	_, err := r.q.ExecContext(ctx, `
		UPDATE product_conversions SET ratio = $2, note = $3 WHERE id = $1 AND is_active = true`,
		id, ratio, note)
	return err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Conversion, error) {
	var cv Conversion
	err := r.q.GetContext(ctx, &cv, selectConversion+` WHERE cv.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &cv, nil
}

func (r *Repository) Create(ctx context.Context, in CreateInput, ratio decimal.Decimal) (int64, error) {
	var id int64
	err := r.q.GetContext(ctx, &id, `
		INSERT INTO product_conversions (from_product_id, to_product_id, ratio, note)
		VALUES ($1,$2,$3,$4) RETURNING id`, in.FromProductID, in.ToProductID, ratio, in.Note)
	return id, err
}

// ListAll returns active + disabled conversions (disabled last), for the admin
// list with "Show disabled" on so they can be reactivated.
func (r *Repository) ListAll(ctx context.Context, search string) ([]Conversion, error) {
	var rows []Conversion
	err := r.q.SelectContext(ctx, &rows, selectConversion+`
		WHERE ($1::text IS NULL
		       OR fp.name ILIKE '%' || $1 || '%'
		       OR fp.name_local ILIKE '%' || $1 || '%'
		       OR tp.name ILIKE '%' || $1 || '%'
		       OR tp.name_local ILIKE '%' || $1 || '%')
		ORDER BY cv.is_active DESC, fp.name`, nullIfBlank(search))
	return rows, err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE product_conversions SET is_active = false WHERE id = $1`, id)
	return err
}

func (r *Repository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.q.ExecContext(ctx, `UPDATE product_conversions SET is_active = true WHERE id = $1`, id)
	return err
}

func (r *Repository) LogRun(ctx context.Context, convID, fromID, toID, userID int64, fromQty, toQty decimal.Decimal) error {
	_, err := r.q.ExecContext(ctx, `
		INSERT INTO conversion_runs (conversion_id, from_product_id, to_product_id, from_qty, to_qty, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)`, convID, fromID, toID, fromQty, toQty, userID)
	return err
}

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

func (s *Service) List(ctx context.Context, search string) ([]Conversion, error) {
	rows, err := s.repo.List(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list conversions", err)
	}
	return rows, nil
}

// ListAll includes disabled conversions (for the admin "Show disabled" toggle).
func (s *Service) ListAll(ctx context.Context, search string) ([]Conversion, error) {
	rows, err := s.repo.ListAll(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list conversions", err)
	}
	return rows, nil
}

// Reactivate re-enables a disabled conversion.
func (s *Service) Reactivate(ctx context.Context, id int64) error {
	if err := s.repo.Reactivate(ctx, id); err != nil {
		return apperr.Internal("failed to reactivate conversion", err)
	}
	return nil
}

// Update edits a conversion's ratio and note.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (*Conversion, error) {
	ratio, err := money.Parse(in.Ratio)
	if err != nil || !ratio.IsPositive() {
		return nil, apperr.Validation("ratio must be greater than zero")
	}
	if err := s.repo.Update(ctx, id, ratio, in.Note); err != nil {
		return nil, apperr.Internal("failed to update conversion", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id int64) (*Conversion, error) {
	cv, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("conversion")
		}
		return nil, apperr.Internal("failed to load conversion", err)
	}
	return cv, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Conversion, error) {
	if in.FromProductID == in.ToProductID {
		return nil, apperr.Validation("source and destination products must differ")
	}
	ratio, err := money.Parse(in.Ratio)
	if err != nil || !ratio.IsPositive() {
		return nil, apperr.Validation("ratio must be greater than zero")
	}
	id, err := s.repo.Create(ctx, in, ratio)
	if err != nil {
		return nil, apperr.Conflict("a conversion between these products already exists")
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperr.Internal("failed to remove conversion", err)
	}
	return nil
}
