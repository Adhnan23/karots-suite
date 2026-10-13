package stock

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ConsumedLot is one batch's contribution to a depletion: which batch, how much
// was taken, what it cost (a zero-cost batch already carries the product's
// cost, see costOfConsumed) and the expiry / batch no it carried.
type ConsumedLot struct {
	BatchID int64
	Qty     decimal.Decimal
	Cost    decimal.Decimal
	Expiry  *time.Time
	BatchNo *string
}

// costOfConsumed returns the weighted-average cost per unit of a depletion.
//
// A lot whose batch cost is zero is charged at productCost instead. Cost is
// frozen into a batch when the batch is created, so stock added before its cost
// was entered — a blank cost box at intake — is stuck at zero even after the
// product's cost is corrected. Left alone it books Rs 0 of COGS and reports the
// whole sale as profit, which is worse than being slightly stale.
//
// The rescue is per lot, not on the final average: one properly-costed batch
// would lift the average above zero and hide the free one. A product whose own
// cost is also zero stays at zero — that one really is free.
func costOfConsumed(lots []ConsumedLot, qty, productCost decimal.Decimal) decimal.Decimal {
	if !qty.IsPositive() {
		return decimal.Zero
	}
	total := decimal.Zero
	for _, l := range lots {
		cost := l.Cost
		if cost.IsZero() {
			cost = productCost
		}
		total = total.Add(l.Qty.Mul(cost))
	}
	return total.Div(qty).Round(2)
}

// DepleteFEFO consumes qty from a product's batches, earliest-expiry-first, and
// returns the weighted-average cost of the consumed units (for COGS snapshots).
// It assumes the caller already passed the atomic stock.quantity guard, so the
// batches should always cover qty; if a rounding shortfall remains it is charged
// against the last touched batch's cost (or zero) rather than failing.
//
// Runs inside the caller's transaction (r.q is the *sqlx.Tx).
func (r *Repository) DepleteFEFO(ctx context.Context, productID int64, qty decimal.Decimal) (decimal.Decimal, error) {
	_, cost, err := r.DepleteFEFOLots(ctx, productID, qty)
	return cost, err
}

// DepleteFEFOLots is DepleteFEFO that also says which batches it took, in FEFO
// order, so a conversion can carry each bag's expiry, batch no and cost into the
// stock it makes. A zero-cost lot is reported at the product's cost (the same
// rescue the average gets).
func (r *Repository) DepleteFEFOLots(ctx context.Context, productID int64, qty decimal.Decimal) ([]ConsumedLot, decimal.Decimal, error) {
	if !qty.IsPositive() {
		return nil, decimal.Zero, nil
	}
	type batchRow struct {
		ID        int64           `db:"id"`
		Remaining decimal.Decimal `db:"qty_remaining"`
		Cost      decimal.Decimal `db:"cost_price"`
		Expiry    *time.Time      `db:"expiry_date"`
		BatchNo   *string         `db:"batch_no"`
	}
	var batches []batchRow
	// FOR UPDATE locks the rows we are about to consume for the tx's lifetime.
	if err := r.q.SelectContext(ctx, &batches, `
		SELECT id, qty_remaining, cost_price, expiry_date, batch_no
		FROM stock_batches
		WHERE product_id = $1 AND qty_remaining > 0
		ORDER BY expiry_date NULLS LAST, id
		FOR UPDATE`, productID); err != nil {
		return nil, decimal.Zero, err
	}

	remaining := qty
	lots := make([]ConsumedLot, 0, len(batches))
	zeroCost := false
	for _, b := range batches {
		if !remaining.IsPositive() {
			break
		}
		take := decimal.Min(b.Remaining, remaining)
		if _, err := r.q.ExecContext(ctx,
			`UPDATE stock_batches SET qty_remaining = qty_remaining - $1 WHERE id = $2`,
			take, b.ID); err != nil {
			return nil, decimal.Zero, err
		}
		lots = append(lots, ConsumedLot{BatchID: b.ID, Qty: take, Cost: b.Cost, Expiry: b.Expiry, BatchNo: b.BatchNo})
		zeroCost = zeroCost || b.Cost.IsZero()
		remaining = remaining.Sub(take)
	}

	// Only pay for the extra lookup when something actually needs rescuing.
	fallback := decimal.Zero
	if zeroCost {
		var err error
		if fallback, err = r.productCost(ctx, productID); err != nil {
			return nil, decimal.Zero, err
		}
		for i := range lots {
			if lots[i].Cost.IsZero() {
				lots[i].Cost = fallback
			}
		}
	}
	return lots, costOfConsumed(lots, qty, fallback), nil
}
