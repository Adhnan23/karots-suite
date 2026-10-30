package sales

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
)

// Flag kinds: something an admin should look at on a sale that was taken
// rather than refused (a Review sale, or one a plugin marked).
const (
	FlagPrice        = "price"         // sold at the price the client showed, not today's
	FlagOversold     = "oversold"      // sold more than the count showed
	FlagOverLimit    = "over_limit"    // credit went past the customer's limit
	FlagLookalike    = "lookalike"     // looks like another sale (rung twice?)
	FlagLate         = "late"          // its session was closed; counted when it arrived
	FlagClock        = "clock"         // the client's clock was off; counted when it arrived
	FlagInactiveUser = "inactive_user" // the cashier was deactivated meanwhile
)

// Flag is one review note on a sale.
type Flag struct {
	ID        int64     `db:"id"         json:"id"`
	SaleID    int64     `db:"sale_id"    json:"sale_id"`
	Kind      string    `db:"kind"       json:"kind"`
	Note      string    `db:"note"       json:"note"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// AddFlag marks a sale for review (inside the caller's tx, or on its own).
func AddFlag(ctx context.Context, q sqlx.ExtContext, saleID int64, kind, note string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO sale_flags (sale_id, kind, note) VALUES ($1,$2,$3)`, saleID, kind, note)
	return err
}

// Flags lists a sale's review notes, oldest first.
func (s *Service) Flags(ctx context.Context, saleID int64) ([]Flag, error) {
	fs := []Flag{}
	if err := s.db.SelectContext(ctx, &fs, `SELECT * FROM sale_flags WHERE sale_id = $1 ORDER BY id`, saleID); err != nil {
		return nil, apperr.Internal("failed to load sale flags", err)
	}
	return fs, nil
}

// byClientRef returns the sale a one-time ref already made, or nil.
func (s *Service) byClientRef(ctx context.Context, ref string) (*Detail, error) {
	if ref == "" {
		return nil, nil
	}
	var id int64
	err := s.db.GetContext(ctx, &id, `SELECT id FROM sales WHERE client_ref = $1`, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal("failed to look up sale", err)
	}
	d, err := s.loadDetail(ctx, s.repo, id)
	if err != nil {
		return nil, err
	}
	d.Duplicate = true
	return d, nil
}

// pendingFlag is a review note found while building the sale, written once the
// sale id exists.
type pendingFlag struct{ kind, note string }

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
