package stock

import (
	"context"

	"karots-suite/internal/apperr"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// LiveLotsFor lists the live batches of several products at once (a stock-take
// page), FEFO order per product.
func (s *Service) LiveLotsFor(ctx context.Context, productIDs []int64) (map[int64][]Batch, error) {
	var rows []Batch
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT b.*, p.name AS product_name, p.selling_price AS product_price, u.abbreviation AS unit_abbr
		FROM stock_batches b
		JOIN products p ON p.id = b.product_id
		JOIN units u    ON u.id = p.unit_id
		WHERE b.product_id = ANY($1) AND b.qty_remaining > 0
		ORDER BY b.product_id, b.expiry_date NULLS LAST, b.id`, pq.Array(productIDs)); err != nil {
		return nil, apperr.Internal("failed to load batches", err)
	}
	out := map[int64][]Batch{}
	for _, b := range rows {
		out[b.ProductID] = append(out[b.ProductID], b)
	}
	return out, nil
}

// SetBatchCost corrects one batch's cost (a count sheet's batch row).
func (s *Service) SetBatchCost(ctx context.Context, batchID int64, cost decimal.Decimal) error {
	if cost.IsNegative() {
		return apperr.Validation("cost cannot be negative")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE stock_batches SET cost_price = $1 WHERE id = $2`, cost, batchID); err != nil {
		return apperr.Internal("failed to update batch cost", err)
	}
	return nil
}
