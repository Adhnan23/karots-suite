// Package documents is Print & Copy: photocopy / print / laminate / bind jobs
// priced from a size × colour × side grid with quantity tiers, using paper by
// size. A print service IS a core service (products.is_service); this package
// only adds the grid, the paper and the job ledger. Core never imports it.
package documents

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/recipes"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// Service is the link row joined to its core service product.
type Service struct {
	ID           int64           `db:"id"`
	ProductID    int64           `db:"product_id"`
	Name         string          `db:"name"`
	Kind         string          `db:"kind"`     // metered | custom
	Category     string          `db:"category"` // copy | print | laminate | bind | other
	IsActive     bool            `db:"is_active"`
	ProductOn    bool            `db:"product_on"`
	SellingPrice decimal.Decimal `db:"selling_price"`
}

var kinds = map[string]bool{"metered": true, "custom": true}
var categories = map[string]bool{"copy": true, "print": true, "laminate": true, "bind": true, "other": true}

const selectService = `
	SELECT s.id, s.product_id, p.name, s.kind, s.category, s.is_active, p.is_active AS product_on, p.selling_price
	FROM doc_service s JOIN products p ON p.id = s.product_id`

func (s *Store) Services(ctx context.Context, activeOnly bool) ([]Service, error) {
	q := selectService
	if activeOnly {
		q += ` WHERE s.is_active AND p.is_active`
	}
	out := []Service{}
	err := s.db.SelectContext(ctx, &out, q+` ORDER BY lower(p.name), s.id`)
	return out, err
}

func (s *Store) Service(ctx context.Context, id int64) (*Service, error) {
	var sv Service
	err := s.db.GetContext(ctx, &sv, selectService+` WHERE s.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("print service")
	}
	return &sv, err
}

func checkKind(kind, category string) error {
	if !kinds[kind] {
		return apperr.Validation("pick metered or custom")
	}
	if !categories[category] {
		return apperr.Validation("pick copy, print, laminate, bind or other")
	}
	return nil
}

// Link makes an existing core service a print service.
func (s *Store) Link(ctx context.Context, q sqlx.QueryerContext, productID int64, kind, category string) (int64, error) {
	if err := checkKind(kind, category); err != nil {
		return 0, err
	}
	var isService bool
	if err := sqlx.GetContext(ctx, q, &isService, `SELECT is_service FROM products WHERE id = $1`, productID); err != nil {
		return 0, apperr.NotFound("service")
	}
	if !isService {
		return 0, apperr.Validation("only a service can be a print service")
	}
	var taken bool
	if err := sqlx.GetContext(ctx, q, &taken, `SELECT EXISTS (SELECT 1 FROM doc_service WHERE product_id = $1)`, productID); err != nil {
		return 0, err
	}
	if taken {
		return 0, apperr.Conflict("that service is already a print service")
	}
	var id int64
	err := sqlx.GetContext(ctx, q, &id, `INSERT INTO doc_service (name, kind, category, product_id)
		SELECT name, $2, $3, id FROM products WHERE id = $1 RETURNING id`, productID, kind, category)
	return id, err
}

func (s *Store) Update(ctx context.Context, id int64, kind, category string) error {
	if err := checkKind(kind, category); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE doc_service SET kind = $2, category = $3 WHERE id = $1`, id, kind, category)
	return err
}

func (s *Store) SetActive(ctx context.Context, id int64, on bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE doc_service SET is_active = $2 WHERE id = $1`, id, on)
	return err
}

func (s *Store) Prices(ctx context.Context, serviceID int64) ([]Price, error) {
	out := []Price{}
	err := s.db.SelectContext(ctx, &out, `SELECT id, service_id, size, material, color, double_side, min_qty, unit_price
		FROM doc_price WHERE service_id = $1 ORDER BY size NULLS FIRST, material, min_qty, color, double_side`, serviceID)
	return out, err
}

func (s *Store) Paper(ctx context.Context, serviceID int64) ([]Paper, error) {
	out := []Paper{}
	err := s.db.SelectContext(ctx, &out, `SELECT dc.id, dc.service_id, dc.size, dc.material, dc.product_id, dc.qty_per_unit,
		       p.name AS product_name, p.cost_price
		FROM doc_consumable dc JOIN products p ON p.id = dc.product_id
		WHERE dc.service_id = $1 ORDER BY dc.size NULLS FIRST, dc.material, dc.id`, serviceID)
	return out, err
}

// SaveGrid replaces a service's prices and paper together.
func (s *Store) SaveGrid(ctx context.Context, serviceID int64, prices []Price, paper []Paper) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{`DELETE FROM doc_price WHERE service_id = $1`, `DELETE FROM doc_consumable WHERE service_id = $1`} {
		if _, err := tx.ExecContext(ctx, q, serviceID); err != nil {
			return err
		}
	}
	for _, p := range prices {
		if _, err := tx.ExecContext(ctx, `INSERT INTO doc_price (service_id, size, material, color, double_side, min_qty, unit_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, serviceID, p.Size, p.Material, p.Color, p.DoubleSide, p.MinQty, p.UnitPrice); err != nil {
			return err
		}
	}
	for _, p := range paper {
		if _, err := tx.ExecContext(ctx, `INSERT INTO doc_consumable (service_id, size, material, product_id, qty_per_unit)
			VALUES ($1, $2, $3, $4, $5)`, serviceID, p.Size, p.Material, p.ProductID, p.QtyPerUnit); err != nil {
			if strings.Contains(err.Error(), "foreign key") {
				return apperr.Validation("pick the paper from your products")
			}
			return err
		}
	}
	return tx.Commit()
}

// Recipe is the service's core recipe (what each copy uses besides paper).
func (s *Store) Recipe(ctx context.Context, productID int64) ([]recipes.Component, error) {
	return recipes.NewRepository(s.db).For(ctx, productID)
}

// Quote is a priced metered job.
type Quote struct {
	Price      Price
	Sheets     int
	Components []recipes.Consumption
	Cost       decimal.Decimal
}

// Quote prices a metered job and lists what it uses.
func (s *Store) Quote(ctx context.Context, sv *Service, o JobOptions) (*Quote, error) {
	if o.Copies <= 0 {
		return nil, apperr.Validation("copies must be more than zero")
	}
	prices, err := s.Prices(ctx, sv.ID)
	if err != nil {
		return nil, err
	}
	pr, ok := PickPrice(prices, o)
	if !ok {
		return nil, apperr.Validation("no price is set for that size, type and option")
	}
	paper, err := s.Paper(ctx, sv.ID)
	if err != nil {
		return nil, err
	}
	rc, err := s.Recipe(ctx, sv.ProductID)
	if err != nil {
		return nil, err
	}
	cs, cost := Components(PaperFor(paper, o.Size, o.Material), rc, o)
	return &Quote{Price: pr, Sheets: Sheets(o), Components: cs, Cost: cost}, nil
}

// Badges pins 🖨 on the till tiles of active print services; the till's
// Print & Copy menu claims a product by it.
func (s *Store) Badges(ctx context.Context, ids []int64) (map[int64][]string, error) {
	out := map[int64][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	q, args, err := sqlx.In(`SELECT product_id FROM doc_service WHERE is_active AND product_id IN (?)`, ids)
	if err != nil {
		return nil, err
	}
	var rows []int64
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(q), args...); err != nil {
		return nil, err
	}
	for _, id := range rows {
		out[id] = []string{Badge}
	}
	return out, nil
}

// Badge marks a print service at the till (the web half matches it).
const Badge = "🖨"
