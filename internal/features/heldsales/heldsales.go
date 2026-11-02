// Package heldsales stores parked/suspended carts so a cashier can hold the
// current sale, serve someone else, and resume it later. The cart payload is
// opaque JSON owned by the terminal; the server only totals/labels it for the
// "held" list and replays it verbatim on resume.
package heldsales

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type HeldSale struct {
	ID         int64           `db:"id"          json:"id"`
	CashierID  int64           `db:"cashier_id"  json:"cashier_id"`
	Label      *string         `db:"label"       json:"label,omitempty"`
	SaleType   string          `db:"sale_type"   json:"sale_type"`
	CustomerID *int64          `db:"customer_id" json:"customer_id,omitempty"`
	Discount   decimal.Decimal `db:"discount"    json:"discount"`
	Cart       json.RawMessage `db:"cart"        json:"cart"`
	ItemCount  int             `db:"item_count"  json:"item_count"`
	Total      decimal.Decimal `db:"total"       json:"total"`
	CreatedAt  time.Time       `db:"created_at"  json:"created_at"`
	// ToCashier: a ticket, waiting for any cashier (CashierID is who sent it).
	ToCashier bool `db:"to_cashier" json:"to_cashier"`
	// joined for a ticket
	SenderName string `db:"sender_name" json:"sender_name,omitempty"`
}

type Input struct {
	Label      string          `json:"label"`
	SaleType   string          `json:"sale_type"`
	CustomerID *int64          `json:"customer_id"`
	Discount   string          `json:"discount"`
	Cart       json.RawMessage `json:"cart" validate:"required"`
	ItemCount  int             `json:"item_count"`
	Total      string          `json:"total"`
	ToCashier  bool            `json:"to_cashier"`
}

type Repository struct{ q appdb.Queryer }

func NewRepository(q appdb.Queryer) *Repository { return &Repository{q: q} }

func (r *Repository) List(ctx context.Context, cashierID int64) ([]HeldSale, error) {
	var rows []HeldSale
	err := r.q.SelectContext(ctx, &rows,
		`SELECT *, '' AS sender_name FROM held_sales WHERE cashier_id = $1 AND NOT to_cashier ORDER BY created_at DESC`, cashierID)
	return rows, err
}

func (r *Repository) Create(ctx context.Context, h HeldSale) (*HeldSale, error) {
	var out HeldSale
	err := r.q.GetContext(ctx, &out, `
		INSERT INTO held_sales (cashier_id, label, sale_type, customer_id, discount, cart, item_count, total, to_cashier)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *, '' AS sender_name`,
		h.CashierID, h.Label, h.SaleType, h.CustomerID, h.Discount, h.Cart, h.ItemCount, h.Total, h.ToCashier)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Tickets lists the tickets waiting, oldest first: only sentBy's when set (a
// sales person's own), else everyone's (a cashier's).
func (r *Repository) Tickets(ctx context.Context, sentBy int64) ([]HeldSale, error) {
	var rows []HeldSale
	err := r.q.SelectContext(ctx, &rows, `SELECT h.*, u.name AS sender_name FROM held_sales h JOIN users u ON u.id = h.cashier_id
		WHERE h.to_cashier AND ($1 = 0 OR h.cashier_id = $1) ORDER BY h.id`, sentBy)
	return rows, err
}

// TakeTicket removes a ticket and returns it, so only one till ever gets it.
func (r *Repository) TakeTicket(ctx context.Context, id, sentBy int64) (*HeldSale, error) {
	var out HeldSale
	err := r.q.GetContext(ctx, &out, `DELETE FROM held_sales h USING users u
		WHERE h.id = $1 AND h.to_cashier AND u.id = h.cashier_id AND ($2 = 0 OR h.cashier_id = $2)
		RETURNING h.*, u.name AS sender_name`, id, sentBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("ticket")
	}
	return &out, err
}

// Delete removes a held sale, scoped to its owner so a cashier can't drop
// another cashier's parked cart.
func (r *Repository) Delete(ctx context.Context, id, cashierID int64) error {
	res, err := r.q.ExecContext(ctx, `DELETE FROM held_sales WHERE id = $1 AND cashier_id = $2`, id, cashierID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("held sale")
	}
	return nil
}

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

func (s *Service) List(ctx context.Context, cashierID int64) ([]HeldSale, error) {
	rows, err := s.repo.List(ctx, cashierID)
	if err != nil {
		return nil, apperr.Internal("failed to list held sales", err)
	}
	return rows, nil
}

func (s *Service) Hold(ctx context.Context, cashierID int64, in Input) (*HeldSale, error) {
	if len(in.Cart) == 0 {
		return nil, apperr.Validation("nothing to hold")
	}
	discount, err := money.Parse(in.Discount)
	if err != nil {
		discount = decimal.Zero
	}
	total, err := money.Parse(in.Total)
	if err != nil {
		total = decimal.Zero
	}
	saleType := in.SaleType
	if saleType == "" {
		saleType = "retail"
	}
	var label *string
	if in.Label != "" {
		label = &in.Label
	}
	h := HeldSale{
		CashierID: cashierID, Label: label, SaleType: saleType, CustomerID: in.CustomerID,
		Discount: discount, Cart: in.Cart, ItemCount: in.ItemCount, Total: total, ToCashier: in.ToCashier,
	}
	out, err := s.repo.Create(ctx, h)
	if err != nil {
		return nil, apperr.Internal("failed to hold sale", err)
	}
	return out, nil
}

func (s *Service) Tickets(ctx context.Context, sentBy int64) ([]HeldSale, error) {
	return s.repo.Tickets(ctx, sentBy)
}

func (s *Service) TakeTicket(ctx context.Context, id, sentBy int64) (*HeldSale, error) {
	return s.repo.TakeTicket(ctx, id, sentBy)
}

func (s *Service) Delete(ctx context.Context, id, cashierID int64) error {
	return s.repo.Delete(ctx, id, cashierID)
}
