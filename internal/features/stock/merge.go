package stock

import (
	"context"
	"sort"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// MergeInput folds BatchIDs into the oldest of them. The prompt sends every
// value it shows: Cost (blank = the weighted average), Expiry (blank = none),
// Price (blank = follows the product).
type MergeInput struct {
	ProductID int64
	BatchIDs  []int64
	Cost      string
	Expiry    string
	Price     string
}

// MergeDefaults is what the merge prompt starts from: the oldest lot (it is
// kept), the quantity-weighted cost to 2dp (stock value unchanged but for
// rounding), the earliest expiry (the safe one) and the oldest lot's own price.
func MergeDefaults(lots []Batch) (keptID int64, cost decimal.Decimal, expiry *time.Time, price decimal.Decimal) {
	if len(lots) == 0 {
		return 0, decimal.Zero, nil, decimal.Zero
	}
	sorted := append([]Batch(nil), lots...)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})
	qty, value := decimal.Zero, decimal.Zero
	for _, b := range sorted {
		qty = qty.Add(b.QtyRemaining)
		value = value.Add(b.QtyRemaining.Mul(b.CostPrice))
		if b.ExpiryDate != nil && (expiry == nil || b.ExpiryDate.Before(*expiry)) {
			expiry = b.ExpiryDate
		}
	}
	if qty.IsPositive() {
		cost = value.Div(qty).Round(2)
	}
	return sorted[0].ID, cost, expiry, sorted[0].SellingPrice
}

// MergeBatches folds lots of one product into the oldest. The others go to 0
// and stay as rows (sale lines and returns still point at them). The count
// doesn't change, so no stock movement is written. Runs on the caller's tx.
func (r *Repository) MergeBatches(ctx context.Context, in MergeInput) (int64, error) {
	seen := map[int64]bool{}
	for _, id := range in.BatchIDs {
		seen[id] = true
	}
	if len(seen) < 2 {
		return 0, apperr.Validation("pick two or more batches to merge")
	}
	var lots []Batch
	if err := r.q.SelectContext(ctx, &lots, `SELECT * FROM stock_batches WHERE id = ANY($1) AND product_id = $2
		ORDER BY id FOR UPDATE`, pq.Array(in.BatchIDs), in.ProductID); err != nil {
		return 0, apperr.Internal("failed to read batches", err)
	}
	if len(lots) != len(seen) {
		return 0, apperr.Validation("a batch picked is not this product's")
	}
	for _, b := range lots {
		if !b.QtyRemaining.IsPositive() {
			return 0, apperr.Validation("a batch picked is already empty")
		}
	}
	kept, cost, _, _ := MergeDefaults(lots)
	if strings.TrimSpace(in.Cost) != "" {
		var err error
		if cost, err = optAmount(in.Cost, "cost"); err != nil {
			return 0, err
		}
	}
	price, err := optAmount(in.Price, "price")
	if err != nil {
		return 0, err
	}
	exp, err := ParseExpiry(in.Expiry)
	if err != nil {
		return 0, err
	}
	moved := decimal.Zero
	for _, b := range lots {
		if b.ID != kept {
			moved = moved.Add(b.QtyRemaining)
		}
	}
	if _, err := r.q.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = 0 WHERE id = ANY($1) AND id <> $2`,
		pq.Array(in.BatchIDs), kept); err != nil {
		return 0, apperr.Internal("failed to empty batches", err)
	}
	if _, err := r.q.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = qty_remaining + $1,
		qty_received = qty_received + $1, cost_price = $2, expiry_date = $3, selling_price = $4 WHERE id = $5`,
		moved, cost, exp, price, kept); err != nil {
		return 0, apperr.Internal("failed to merge batches", err)
	}
	return kept, nil
}

// MergeBatches runs Repository.MergeBatches in its own transaction.
func (s *Service) MergeBatches(ctx context.Context, in MergeInput) (id int64, err error) {
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		id, err = NewRepository(tx).MergeBatches(ctx, in)
		return err
	})
	return id, err
}
