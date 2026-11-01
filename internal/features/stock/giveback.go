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

// GiveBackInput is a lot going back to its supplier. Counted is what's really
// on the shelf; it wins over the system's number.
type GiveBackInput struct {
	BatchID      int64
	Counted      string
	SupplierName string
}

// GiveBack hands a counted lot to its supplier and empties it. Batches get mixed
// on the shelf, so a difference moves between the product's lots (next to sell
// first); only what they can't take changes the total, as an adjustment. The
// hand-over itself is a damage write-off marked given_back, which waits on the
// supplier until settled. Answers its loss; 0 when nothing was counted.
func (s *Service) GiveBack(ctx context.Context, in GiveBackInput, userID int64) (loss int64, err error) {
	text := strings.TrimSpace(in.Counted)
	counted, err := money.Parse(text)
	if text == "" || err != nil || counted.IsNegative() {
		return 0, apperr.Validation("enter what you counted on the shelf")
	}
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		var lot Batch
		if err := tx.GetContext(ctx, &lot, `SELECT * FROM stock_batches WHERE id = $1 FOR UPDATE`, in.BatchID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("batch")
			}
			return apperr.Internal("failed to read batch", err)
		}
		if !lot.QtyRemaining.IsPositive() {
			return apperr.Conflict("that batch has already gone")
		}
		if err := repo.settleCount(ctx, tx, lot, counted, userID); err != nil {
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
		if _, err := tx.ExecContext(ctx, `UPDATE stock_movements SET given_back = true WHERE id = $1`, loss); err != nil {
			return apperr.Internal("failed to mark the hand-over", err)
		}
		return nil
	})
	return loss, err
}

// settleCount leaves the lot holding exactly counted, moving the difference to
// or from the product's other lots, and makes sure the total can cover it.
func (r *Repository) settleCount(ctx context.Context, tx *sqlx.Tx, lot Batch, counted decimal.Decimal, userID int64) error {
	var others []Batch
	if err := tx.SelectContext(ctx, &others, `SELECT * FROM stock_batches WHERE product_id = $1 AND id <> $2 AND qty_remaining > 0
		ORDER BY expiry_date NULLS LAST, id FOR UPDATE`, lot.ProductID, lot.ID); err != nil {
		return apperr.Internal("failed to read batches", err)
	}
	setLot := func(id int64, qty decimal.Decimal) error {
		if _, err := tx.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = $1 WHERE id = $2`, qty, id); err != nil {
			return apperr.Internal("failed to update batch", err)
		}
		return nil
	}
	moved := func(qty decimal.Decimal, from, to int64) error {
		note := fmt.Sprintf("%s moved from batch %d to batch %d: counted when giving back", qty, from, to)
		return r.logMovement(ctx, lot.ProductID, decimal.Zero, decimal.Zero, note, userID)
	}
	adjust := decimal.Zero // what the lots couldn't take, up or down
	switch diff := counted.Sub(lot.QtyRemaining); {
	case diff.IsPositive():
		need := diff
		for _, o := range others {
			if !need.IsPositive() {
				break
			}
			take := decimal.Min(need, o.QtyRemaining)
			if err := setLot(o.ID, o.QtyRemaining.Sub(take)); err != nil {
				return err
			}
			if err := moved(take, o.ID, lot.ID); err != nil {
				return err
			}
			need = need.Sub(take)
		}
		adjust = need
	case diff.IsNegative() && len(others) > 0:
		if err := setLot(others[0].ID, others[0].QtyRemaining.Add(diff.Abs())); err != nil {
			return err
		}
		if err := moved(diff.Abs(), lot.ID, others[0].ID); err != nil {
			return err
		}
	case diff.IsNegative():
		adjust = diff
	}
	if err := setLot(lot.ID, counted); err != nil {
		return err
	}
	var total decimal.Decimal
	if err := tx.GetContext(ctx, &total, `SELECT COALESCE((SELECT quantity FROM stock WHERE product_id = $1 FOR UPDATE), 0)`, lot.ProductID); err != nil {
		return apperr.Internal("failed to read stock", err)
	}
	// The total can drift from the lots; a hand-over never stops for that.
	if short := counted.Sub(total.Add(adjust)); short.IsPositive() {
		adjust = adjust.Add(short)
	}
	if adjust.IsZero() {
		return nil
	}
	if err := r.Increment(ctx, lot.ProductID, adjust); err != nil {
		return apperr.Internal("failed to update stock", err)
	}
	return r.logMovement(ctx, lot.ProductID, adjust, lot.CostPrice.Mul(adjust.Abs()), "counted when giving back", userID)
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

// SupplierLots lists a supplier's live lots that have an expiry date, soonest first.
func (s *Service) SupplierLots(ctx context.Context, supplierID int64) ([]Batch, error) {
	var rows []Batch
	if err := s.db.SelectContext(ctx, &rows, lotSelect+`
		WHERE b.qty_remaining > 0 AND b.expiry_date IS NOT NULL AND `+LotSupplierSQL+` = $1
		ORDER BY b.expiry_date, b.id`, supplierID); err != nil {
		return nil, apperr.Internal("failed to load batches", err)
	}
	return rows, nil
}
