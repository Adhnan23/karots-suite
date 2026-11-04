package stock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// GiveBackInput is a lot going back to a supplier. Counted is what's really on
// the shelf; it wins over the system's number.
type GiveBackInput struct {
	BatchID      int64
	Counted      string
	SupplierID   int64
	SupplierName string
}

// GiveBack hands a counted lot to its supplier and empties it. Batches get mixed
// on the shelf, so a difference moves between the product's lots (next to sell
// first); only what they can't take changes the total, as an adjustment. The
// hand-over itself is a damage write-off that remembers the supplier, and waits
// on them until settled. Answers its loss; 0 when nothing was counted.
func (s *Service) GiveBack(ctx context.Context, in GiveBackInput, userID int64) (loss int64, err error) {
	text := strings.TrimSpace(in.Counted)
	counted, err := money.Parse(text)
	if text == "" || err != nil || counted.IsNegative() {
		return 0, apperr.Validation("enter what you counted on the shelf")
	}
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		var productID int64
		if err := tx.GetContext(ctx, &productID, `SELECT product_id FROM stock_batches WHERE id = $1`, in.BatchID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("batch")
			}
			return apperr.Internal("failed to read batch", err)
		}
		// The stock row first, then the lots: the order a sale takes them in.
		var total decimal.Decimal
		if err := tx.GetContext(ctx, &total, `SELECT COALESCE((SELECT quantity FROM stock WHERE product_id = $1 FOR UPDATE), 0)`, productID); err != nil {
			return apperr.Internal("failed to read stock", err)
		}
		var lot Batch
		if err := tx.GetContext(ctx, &lot, `SELECT * FROM stock_batches WHERE id = $1 FOR UPDATE`, in.BatchID); err != nil {
			return apperr.Internal("failed to read batch", err)
		}
		if !lot.QtyRemaining.IsPositive() {
			return apperr.Conflict("that batch has already gone")
		}
		if err := repo.settleCount(ctx, tx, lot, counted, total, userID); err != nil {
			return err
		}
		if !counted.IsPositive() {
			return nil
		}
		loss, err = consumeTx(ctx, tx, ConsumeInput{ProductID: lot.ProductID, Quantity: counted.String(), Reason: "damage",
			BatchID: lot.ID, Note: "given back to " + in.SupplierName}, userID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE stock_movements SET given_back_to = $1 WHERE id = $2`, in.SupplierID, loss); err != nil {
			return apperr.Internal("failed to mark the hand-over", err)
		}
		return nil
	})
	return loss, err
}

// settleCount leaves the lot holding exactly counted, moving the difference to
// or from the product's other lots at a blended cost so no value appears or goes
// missing, and makes sure the total can cover the count.
func (r *Repository) settleCount(ctx context.Context, tx *sqlx.Tx, lot Batch, counted, total decimal.Decimal, userID int64) error {
	var others []Batch
	if err := tx.SelectContext(ctx, &others, `SELECT * FROM stock_batches WHERE product_id = $1 AND id <> $2 AND qty_remaining > 0
		ORDER BY expiry_date NULLS LAST, id FOR UPDATE`, lot.ProductID, lot.ID); err != nil {
		return apperr.Internal("failed to read batches", err)
	}
	productCost, err := r.productCost(ctx, lot.ProductID)
	if err != nil {
		return apperr.Internal("failed to read cost", err)
	}
	// A lot with a blank cost is worth the product's cost, as when it's sold.
	costOf := func(b Batch) decimal.Decimal {
		if b.CostPrice.IsZero() {
			return productCost
		}
		return b.CostPrice
	}
	setLot := func(id int64, qty, cost decimal.Decimal) error {
		if _, err := tx.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = $1, cost_price = $2 WHERE id = $3`, qty, cost, id); err != nil {
			return apperr.Internal("failed to update batch", err)
		}
		return nil
	}
	moved := func(qty decimal.Decimal, from, to int64) error {
		note := fmt.Sprintf("%s moved from batch %d to batch %d: counted when giving back", qty, from, to)
		return r.logMovement(ctx, lot.ProductID, decimal.Zero, decimal.Zero, note, userID)
	}
	lotCost := costOf(lot)
	adjust := decimal.Zero // units missing with no other lot to take them
	switch diff := counted.Sub(lot.QtyRemaining); {
	case diff.IsPositive():
		need, held, value := diff, lot.QtyRemaining, lot.QtyRemaining.Mul(lotCost)
		for _, o := range others {
			if !need.IsPositive() {
				break
			}
			take := decimal.Min(need, o.QtyRemaining)
			if err := setLot(o.ID, o.QtyRemaining.Sub(take), o.CostPrice); err != nil {
				return err
			}
			if err := moved(take, o.ID, lot.ID); err != nil {
				return err
			}
			need, held, value = need.Sub(take), held.Add(take), value.Add(take.Mul(costOf(o)))
		}
		lotCost = value.Div(held).Round(4)
	case diff.IsNegative() && len(others) > 0:
		o, out := others[0], diff.Abs()
		blended := o.QtyRemaining.Mul(costOf(o)).Add(out.Mul(lotCost)).Div(o.QtyRemaining.Add(out)).Round(4)
		if err := setLot(o.ID, o.QtyRemaining.Add(out), blended); err != nil {
			return err
		}
		if err := moved(out, lot.ID, o.ID); err != nil {
			return err
		}
	case diff.IsNegative():
		adjust = diff
	}
	if err := setLot(lot.ID, counted, lotCost); err != nil {
		return err
	}
	// More than the lots held comes from what the total holds outside them; only
	// past that is it found stock. A hand-over never stops for the difference.
	if short := counted.Sub(total.Add(adjust)); short.IsPositive() {
		adjust = adjust.Add(short)
	}
	if adjust.IsZero() {
		return nil
	}
	if err := r.Increment(ctx, lot.ProductID, adjust); err != nil {
		return apperr.Internal("failed to update stock", err)
	}
	return r.logMovement(ctx, lot.ProductID, adjust, lotCost.Mul(adjust.Abs()), "counted when giving back", userID)
}

func (r *Repository) logMovement(ctx context.Context, productID int64, qty, cost decimal.Decimal, note string, userID int64) error {
	if err := r.InsertMovement(ctx, MovementInput{ProductID: productID, Type: MoveAdjust, Quantity: qty, UserID: userID, Note: &note, Cost: cost}); err != nil {
		return apperr.Internal("failed to record movement", err)
	}
	return nil
}

// MarkSupplierTold records that the lot's supplier was told about it, today.
func (s *Service) MarkSupplierTold(ctx context.Context, batchID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE stock_batches SET supplier_told_on = CURRENT_DATE WHERE id = $1`, batchID)
	if err != nil {
		return apperr.Internal("failed to record it", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("batch")
	}
	return nil
}

// RecallLot marks a lot recalled by its supplier, today. stopSelling makes the
// till warn before selling it; either way it stays in stock until given back.
func (s *Service) RecallLot(ctx context.Context, batchID int64, stopSelling bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE stock_batches SET recalled_on = CURRENT_DATE, recall_stops = $1 WHERE id = $2`, stopSelling, batchID)
	if err != nil {
		return apperr.Internal("failed to recall the batch", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("batch")
	}
	return nil
}

// UndoRecall puts a recalled lot back to normal.
func (s *Service) UndoRecall(ctx context.Context, batchID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE stock_batches SET recalled_on = NULL, recall_stops = false WHERE id = $1`, batchID)
	if err != nil {
		return apperr.Internal("failed to undo the recall", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("batch")
	}
	return nil
}

// SupplierLots lists a supplier's live lots that have an expiry date or are
// recalled, soonest first.
func (s *Service) SupplierLots(ctx context.Context, supplierID int64) ([]Batch, error) {
	var rows []Batch
	if err := s.db.SelectContext(ctx, &rows, lotSelect+`
		WHERE b.qty_remaining > 0 AND (b.expiry_date IS NOT NULL OR b.recalled_on IS NOT NULL) AND `+LotSupplierSQL+` = $1
		ORDER BY b.expiry_date NULLS LAST, b.id`, supplierID); err != nil {
		return nil, apperr.Internal("failed to load batches", err)
	}
	return rows, nil
}
