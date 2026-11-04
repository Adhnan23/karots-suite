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
	"slices"
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
	ToCashier  bool       `db:"to_cashier"  json:"to_cashier"`
	TicketNo   *int       `db:"ticket_no"   json:"ticket_no,omitempty"`
	ForCashier *int64     `db:"for_cashier" json:"for_cashier,omitempty"`
	AssignedAt *time.Time `db:"assigned_at" json:"assigned_at,omitempty"`
	// joined for a ticket
	SenderName string  `db:"sender_name" json:"sender_name,omitempty"`
	ForName    *string `db:"for_name"    json:"for_name,omitempty"`
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
	ForCashier *int64          `json:"for_cashier"` // nil = the one cashier open
}

// Cashier is someone with a till open: who a ticket can go to.
type Cashier struct {
	ID   int64  `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}

// moveAfter is how long a ticket waits on one till before it goes to the next.
const moveAfter = "2 minutes"

const ticketSelect = `SELECT h.*, u.name AS sender_name, f.name AS for_name FROM held_sales h
	JOIN users u ON u.id = h.cashier_id LEFT JOIN users f ON f.id = h.for_cashier`

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
		INSERT INTO held_sales (cashier_id, label, sale_type, customer_id, discount, cart, item_count, total, to_cashier, for_cashier,
			ticket_no, assigned_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::bigint,
			CASE WHEN $9 THEN COALESCE(
				(SELECT n FROM generate_series(10, 99) n WHERE n NOT IN (SELECT ticket_no FROM held_sales WHERE to_cashier) ORDER BY random() LIMIT 1),
				(SELECT max(ticket_no) + 1 FROM held_sales WHERE to_cashier)) END,
			CASE WHEN $9 THEN now() END)
		RETURNING *`,
		h.CashierID, h.Label, h.SaleType, h.CustomerID, h.Discount, h.Cart, h.ItemCount, h.Total, h.ToCashier, h.ForCashier)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Tickets lists the tickets waiting, oldest first: only sentBy's when set (a
// sales person's own), else everyone's (a cashier's).
func (r *Repository) Tickets(ctx context.Context, sentBy int64) ([]HeldSale, error) {
	var rows []HeldSale
	err := r.q.SelectContext(ctx, &rows, ticketSelect+`
		WHERE h.to_cashier AND ($1 = 0 OR h.cashier_id = $1) ORDER BY h.id`, sentBy)
	return rows, err
}

// Cashiers lists who can take a ticket: working people with a till open, not a
// sales person (one can hold a till a phone opened for a late sale).
func (r *Repository) Cashiers(ctx context.Context) ([]Cashier, error) {
	rows := []Cashier{}
	err := r.q.SelectContext(ctx, &rows, `SELECT DISTINCT u.id, u.name FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE cr.closed_at IS NULL AND u.is_active AND NOT u.sends_to_cashier ORDER BY u.name`)
	return rows, err
}

// MoveUnanswered sends a ticket on to the next open till when its own has left it
// waiting too long or has closed. With no till left at all it goes back to the
// sender's Held, so nothing waits unseen until the next opening.
func (r *Repository) MoveUnanswered(ctx context.Context) error {
	if _, err := r.q.ExecContext(ctx, `WITH open AS (SELECT DISTINCT cr.user_id AS id FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE cr.closed_at IS NULL AND u.is_active AND NOT u.sends_to_cashier)
		UPDATE held_sales SET to_cashier = false, ticket_no = NULL, for_cashier = NULL, assigned_at = NULL
		WHERE to_cashier AND NOT EXISTS (SELECT 1 FROM open)`); err != nil {
		return err
	}
	_, err := r.q.ExecContext(ctx, `WITH open AS (SELECT DISTINCT cr.user_id AS id FROM cash_register cr JOIN users u ON u.id = cr.user_id
		WHERE cr.closed_at IS NULL AND u.is_active AND NOT u.sends_to_cashier)
		UPDATE held_sales h SET assigned_at = now(), for_cashier = COALESCE(
			(SELECT min(id) FROM open WHERE id > h.for_cashier), (SELECT min(id) FROM open))
		WHERE h.to_cashier AND EXISTS (SELECT 1 FROM open WHERE id <> COALESCE(h.for_cashier, 0))
			AND (h.assigned_at < now() - interval '`+moveAfter+`' OR h.for_cashier IS NULL OR h.for_cashier NOT IN (SELECT id FROM open))`)
	return err
}

// WaitingFor counts the tickets on a cashier's till.
func (r *Repository) WaitingFor(ctx context.Context, cashierID int64) (int, error) {
	var n int
	err := r.q.GetContext(ctx, &n, `SELECT count(*) FROM held_sales WHERE to_cashier AND for_cashier = $1`, cashierID)
	return n, err
}

// Pass gives a ticket to another cashier, its two minutes starting again.
func (r *Repository) Pass(ctx context.Context, id, to int64) error {
	res, err := r.q.ExecContext(ctx, `UPDATE held_sales SET for_cashier = $2, assigned_at = now() WHERE id = $1 AND to_cashier`, id, to)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("ticket")
	}
	return nil
}

// BackToSender makes a ticket a held sale of the person who sent it.
func (r *Repository) BackToSender(ctx context.Context, id int64) error {
	res, err := r.q.ExecContext(ctx, `UPDATE held_sales SET to_cashier = false, ticket_no = NULL, for_cashier = NULL, assigned_at = NULL
		WHERE id = $1 AND to_cashier`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("ticket")
	}
	return nil
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
	if in.ToCashier {
		if in.ForCashier, err = s.target(ctx, in.ForCashier); err != nil {
			return nil, err
		}
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
		Discount: discount, Cart: in.Cart, ItemCount: in.ItemCount, Total: total, ToCashier: in.ToCashier, ForCashier: in.ForCashier,
	}
	for try := 0; ; try++ {
		out, err := s.repo.Create(ctx, h)
		if err == nil {
			return out, nil
		}
		if try == 4 || !appdb.IsUniqueViolation(err) { // two sends drew one number: draw again
			return nil, apperr.Internal("failed to hold sale", err)
		}
	}
}

// target checks a ticket's cashier has a till open; with none named, the only one open.
func (s *Service) target(ctx context.Context, to *int64) (*int64, error) {
	open, err := s.repo.Cashiers(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list open tills", err)
	}
	if len(open) == 0 {
		return nil, apperr.Conflict("no cashier has opened a till — hold this sale for now")
	}
	if to == nil {
		if len(open) > 1 {
			return nil, apperr.Validation("choose a cashier")
		}
		return &open[0].ID, nil
	}
	if !slices.ContainsFunc(open, func(c Cashier) bool { return c.ID == *to }) {
		return nil, apperr.Conflict("that cashier's till isn't open")
	}
	return to, nil
}

func (s *Service) Cashiers(ctx context.Context) ([]Cashier, error) { return s.repo.Cashiers(ctx) }

func (s *Service) Tickets(ctx context.Context, sentBy int64) ([]HeldSale, error) {
	if err := s.repo.MoveUnanswered(ctx); err != nil {
		return nil, apperr.Internal("failed to move tickets", err)
	}
	return s.repo.Tickets(ctx, sentBy)
}

// CheckNoTickets refuses a till's own close while tickets wait on it (the screen
// lists them; one can arrive just after it last looked).
func (s *Service) CheckNoTickets(ctx context.Context, cashierID int64) error {
	n, err := s.repo.WaitingFor(ctx, cashierID)
	if err != nil {
		return apperr.Internal("failed to check tickets", err)
	}
	if n > 0 {
		return apperr.ConflictCode("TICKETS_WAITING", "a ticket has just arrived on this till — deal with it before closing")
	}
	return nil
}

func (s *Service) PassTicket(ctx context.Context, id, to int64) error {
	if _, err := s.target(ctx, &to); err != nil {
		return err
	}
	return s.repo.Pass(ctx, id, to)
}

func (s *Service) TicketBackToSender(ctx context.Context, id int64) error {
	return s.repo.BackToSender(ctx, id)
}

func (s *Service) TakeTicket(ctx context.Context, id, sentBy int64) (*HeldSale, error) {
	return s.repo.TakeTicket(ctx, id, sentBy)
}

func (s *Service) Delete(ctx context.Context, id, cashierID int64) error {
	return s.repo.Delete(ctx, id, cashierID)
}
