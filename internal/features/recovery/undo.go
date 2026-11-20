package recovery

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Undo takes a recovery back exactly, while nothing came after it (REC-8): a
// replacement while no stock of its product moved since (its units leave the
// lot at their worth, a top-up's lot gets its cost back); a credit or an extra
// charge while nothing since touched the supplier's books (the bills and old
// debt it went on owe that again); written off always. The loss then waits on
// the claims list again. Locks as Record: the loss, the recovery, stock, the lot,
// the supplier last.
func (s *Service) Undo(ctx context.Context, id, userID int64) (*Recovery, error) {
	var head undoRow
	if err := s.db.GetContext(ctx, &head, `SELECT * FROM loss_recoveries WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("recovery")
		}
		return nil, apperr.Internal("failed to load recovery", err)
	}
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := NewRepository(tx).lockSource(ctx, head.SourceType, head.SourceID); err != nil {
			return apperr.Internal("failed to load loss", err)
		}
		var r undoRow
		if err := tx.GetContext(ctx, &r, `SELECT * FROM loss_recoveries WHERE id = $1 FOR UPDATE`, id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("recovery")
			}
			return apperr.Internal("failed to load recovery", err)
		}
		switch r.Outcome {
		case OutcomeReplacement:
			if err := undoReplacement(ctx, tx, &r, userID); err != nil {
				return err
			}
		case OutcomePaid:
			if r.OldDebt == nil {
				return apperr.Conflict("this credit was recorded before undo existed")
			}
			if err := holdUnchanged(ctx, tx, &r); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE purchases p SET credited_amount = p.credited_amount - b.amount,
				       status = (CASE WHEN p.paid_amount + p.credited_amount - b.amount >= p.total THEN 'paid'
				                      WHEN p.paid_amount > 0 THEN 'partial' ELSE 'received' END)::purchase_status
				FROM loss_recovery_bills b WHERE b.recovery_id = $1 AND p.id = b.purchase_id`, r.ID); err != nil {
				return apperr.Internal("failed to put the bills back", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE suppliers SET outstanding_balance = outstanding_balance + $2,
				opening_unlinked = opening_unlinked + $3 WHERE id = $1`, *r.SupplierID, r.RecoveredValue, *r.OldDebt); err != nil {
				return apperr.Internal("failed to put the balance back", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM loss_recoveries WHERE id = $1`, r.ID); err != nil {
			return apperr.Internal("failed to remove the recovery", err)
		}
		head = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &head.Recovery, nil
}

// undoRow is a recovery with what it needs to be undone.
type undoRow struct {
	Recovery
	LotID         *int64           `db:"lot_id"`
	LotCostBefore *decimal.Decimal `db:"lot_cost_before"`
	OldDebt       *decimal.Decimal `db:"old_debt"`
}

func undoReplacement(ctx context.Context, tx *sqlx.Tx, r *undoRow, userID int64) error {
	var mv struct {
		ID   int64           `db:"id"`
		Cost decimal.Decimal `db:"cost"`
	}
	if err := tx.GetContext(ctx, &mv, `SELECT id, cost FROM stock_movements
		WHERE type = 'recovery' AND reference_type = 'recovery' AND reference_id = $1 AND quantity > 0`, r.ID); err != nil || r.LotID == nil {
		return apperr.Conflict("this replacement was recorded before undo existed")
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM stock WHERE product_id = $1 FOR UPDATE`, r.ProductID); err != nil {
		return apperr.Internal("failed to hold stock", err)
	}
	var later int
	if err := tx.GetContext(ctx, &later, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND id > $2`, r.ProductID, mv.ID); err != nil {
		return apperr.Internal("failed to read stock", err)
	}
	if later > 0 {
		return apperr.Conflict("stock of this product has moved since the replacement came in — undo it by hand")
	}
	res, err := tx.ExecContext(ctx, `UPDATE stock_batches SET qty_remaining = qty_remaining - $2, qty_received = qty_received - $2,
		cost_price = COALESCE($3, cost_price) WHERE id = $1 AND qty_remaining >= $2`, *r.LotID, r.Quantity, r.LotCostBefore)
	if err != nil {
		return apperr.Internal("failed to take the units back", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return apperr.Conflict("the replaced units are no longer in their batch")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stock SET quantity = quantity - $2, last_updated = NOW() WHERE product_id = $1`, r.ProductID, r.Quantity); err != nil {
		return apperr.Internal("failed to update stock", err)
	}
	note := "supplier replacement undone"
	if _, err := tx.ExecContext(ctx, `INSERT INTO stock_movements (product_id, type, quantity, reference_type, reference_id, user_id, note, cost)
		VALUES ($1, 'recovery', $2, 'recovery', $3, $4, $5, $6)`, r.ProductID, r.Quantity.Neg(), r.ID, userID, note, mv.Cost); err != nil {
		return apperr.Internal("failed to record stock movement", err)
	}
	if r.ExtraCharge.IsPositive() {
		if err := holdUnchanged(ctx, tx, r); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE suppliers SET outstanding_balance = outstanding_balance - $2,
			opening_unlinked = LEAST(opening_unlinked, GREATEST(outstanding_balance - $2, 0)) WHERE id = $1`, *r.SupplierID, r.ExtraCharge); err != nil {
			return apperr.Internal("failed to take the charge back", err)
		}
	}
	return nil
}

// holdUnchanged holds the recovery's supplier and refuses when anything since
// touched their books: a bill, a payment, goods sent back, a refund, another
// recovery.
func holdUnchanged(ctx context.Context, tx *sqlx.Tx, r *undoRow) error {
	if _, err := lockSupplier(ctx, tx, *r.SupplierID); err != nil {
		return err
	}
	var later bool
	if err := tx.GetContext(ctx, &later, `SELECT
		EXISTS (SELECT 1 FROM purchases WHERE supplier_id = $1 AND status <> 'draft' AND created_at > $2)
		OR EXISTS (SELECT 1 FROM supplier_payments WHERE supplier_id = $1 AND created_at > $2)
		OR EXISTS (SELECT 1 FROM purchase_returns WHERE supplier_id = $1 AND created_at > $2)
		OR EXISTS (SELECT 1 FROM supplier_refunds WHERE supplier_id = $1 AND created_at > $2)
		OR EXISTS (SELECT 1 FROM loss_recoveries WHERE supplier_id = $1 AND id > $3)`, *r.SupplierID, r.CreatedAt, r.ID); err != nil {
		return apperr.Internal("failed to read the supplier's books", err)
	}
	if later {
		return apperr.Conflict("the supplier's books have moved since — undo it by hand (a refund or a payment)")
	}
	return nil
}
