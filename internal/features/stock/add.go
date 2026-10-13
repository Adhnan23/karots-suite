package stock

import (
	"cmp"
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

// AddStockInput adds Qty to a product. BatchID > 0 tops up that batch (its cost,
// price and expiry stay); 0 opens a new batch with Cost (blank = the product's),
// Price (blank = follows the product) and Expiry (YYYY-MM-DD, blank = none).
type AddStockInput struct {
	ProductID int64
	BatchID   int64
	Qty       string
	Cost      string
	Price     string
	Expiry    string
	Note      string
	// Source / Move / RefType name a new batch's source and the movement
	// (default "adjust"); a supplier replacement uses "recovery".
	Source  string
	Move    string
	RefType *string
}

// ParseExpiry reads a YYYY-MM-DD expiry; blank is no expiry.
func ParseExpiry(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, apperr.Validation("expiry must be a date (YYYY-MM-DD)")
	}
	return &d, nil
}

// optAmount parses a blank-able non-negative amount (blank = zero).
func optAmount(s, what string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	v, err := money.Parse(s)
	if err != nil || v.IsNegative() {
		return decimal.Zero, apperr.Validation(what + " must be a non-negative amount")
	}
	return v, nil
}

// AddStock is the one way units come in outside a purchase or a return:
// intake, Batches → + Add batch, stock-take's new batches. Returns the batch
// the units went into. Runs on the caller's transaction (r.q).
func (r *Repository) AddStock(ctx context.Context, in AddStockInput, userID int64) (int64, error) {
	qty, err := money.Parse(strings.TrimSpace(in.Qty))
	if err != nil || !qty.IsPositive() {
		return 0, apperr.Validation("enter a quantity to add")
	}
	var cost decimal.Decimal
	batchID := in.BatchID
	if batchID > 0 {
		b, err := r.LockBatch(ctx, batchID, in.ProductID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, apperr.Validation("that batch is not this product's")
			}
			return 0, apperr.Internal("failed to read batch", err)
		}
		if _, err := r.q.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = qty_remaining + $1,
			qty_received = qty_received + $1 WHERE id = $2`, qty, batchID); err != nil {
			return 0, apperr.Internal("failed to top up batch", err)
		}
		cost = b.CostPrice
	} else {
		if cost, err = optAmount(in.Cost, "cost"); err != nil {
			return 0, err
		}
		price, err := optAmount(in.Price, "price")
		if err != nil {
			return 0, err
		}
		exp, err := ParseExpiry(in.Expiry)
		if err != nil {
			return 0, err
		}
		if batchID, err = r.InsertBatch(ctx, NewBatch{ProductID: in.ProductID, Quantity: qty, CostPrice: cost,
			SellingPrice: price, ExpiryDate: exp, Source: cmp.Or(in.Source, "adjust")}); err != nil {
			return 0, apperr.Internal("failed to open batch", err)
		}
		// InsertBatch fills a blank cost from the product; value the movement the same.
		if err := r.q.GetContext(ctx, &cost, `SELECT cost_price FROM stock_batches WHERE id = $1`, batchID); err != nil {
			return 0, apperr.Internal("failed to read batch", err)
		}
	}
	if _, err := r.q.ExecContext(ctx, `INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET quantity = stock.quantity + EXCLUDED.quantity, last_updated = NOW()`,
		in.ProductID, qty); err != nil {
		return 0, apperr.Internal("failed to update stock", err)
	}
	if err := r.InsertMovement(ctx, MovementInput{ProductID: in.ProductID, Type: cmp.Or(in.Move, MoveAdjust), Quantity: qty, ReferenceType: in.RefType,
		UserID: userID, Note: nilIfEmpty(in.Note), Cost: cost.Mul(qty)}); err != nil {
		return 0, apperr.Internal("failed to record movement", err)
	}
	return batchID, nil
}

// AddStock runs Repository.AddStock in its own transaction.
func (s *Service) AddStock(ctx context.Context, in AddStockInput, userID int64) (id int64, err error) {
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		id, err = NewRepository(tx).AddStock(ctx, in, userID)
		return err
	})
	return id, err
}
