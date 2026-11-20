package recovery

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

// SourceInfo is the product name + worth of a loss, for pre-filling the form.
type SourceInfo struct {
	ProductName string
	LossValue   decimal.Decimal
}

// Record books one recovery against a loss, in a single transaction:
//   - replacement: the supplier handed back a unit → restock it (a recovery
//     movement) and count its worth as recovered.
//   - paid:        the supplier paid/credited → lower their payable and record
//     the cash recovered.
//   - written_off: nothing recovered; the loss stands.
func (s *Service) Record(ctx context.Context, in CreateInput, userID int64) error {
	if in.SourceType != SourceWarranty && in.SourceType != SourceDamage {
		return apperr.BadRequest("invalid loss source")
	}
	if in.Outcome != OutcomeReplacement && in.Outcome != OutcomePaid && in.Outcome != OutcomeWrittenOff {
		return apperr.Validation("choose an outcome")
	}

	var paidAmount decimal.Decimal
	if in.Outcome == OutcomePaid {
		amt, err := money.Parse(in.RecoveredAmount)
		if err != nil || !amt.IsPositive() {
			return apperr.Validation("enter the amount the supplier paid")
		}
		if in.SupplierID == nil || *in.SupplierID <= 0 {
			return apperr.Validation("select the supplier who paid")
		}
		paidAmount = amt
	}
	extra := decimal.Zero
	if in.Outcome == OutcomeReplacement && strings.TrimSpace(in.ExtraPerUnit) != "" {
		v, err := money.Parse(in.ExtraPerUnit)
		if err != nil || v.IsNegative() {
			return apperr.Validation("the extra must be an amount (0 if the replacement is free)")
		}
		if v.IsPositive() && (in.SupplierID == nil || *in.SupplierID <= 0) {
			return apperr.Validation("select the supplier who asked for the extra")
		}
		extra = v
	}

	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		// The loss first: it is held, so one loss is settled once (REC-1).
		src, err := repo.lockSource(ctx, in.SourceType, in.SourceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("loss")
			}
			return apperr.Internal("failed to load loss", err)
		}
		if in.SourceType == SourceWarranty && src.Status != "replaced" {
			return apperr.Conflict("only a replaced unit can be recovered from the supplier")
		}
		already, err := repo.RecoveredQtyForSource(ctx, in.SourceType, in.SourceID)
		if err != nil {
			return apperr.Internal("failed to check existing recovery", err)
		}
		if already.GreaterThanOrEqual(src.Quantity) {
			return apperr.Conflict("this loss has already been recovered")
		}

		rec := Recovery{SourceType: in.SourceType, SourceID: in.SourceID, ProductID: src.ProductID, SupplierID: in.SupplierID,
			Outcome: in.Outcome, Quantity: src.Quantity, LossValue: src.LossValue, Note: nilIfEmpty(in.Note), HandledBy: userID}
		switch in.Outcome {
		case OutcomeReplacement:
			// The goods came back, so the loss is recovered in kind; any extra the
			// supplier asked on top is owed to them.
			rec.RecoveredValue = src.LossValue
			rec.ExtraCharge = extra.Mul(src.Quantity).Round(2)
		case OutcomePaid:
			rec.RecoveredValue = paidAmount
		}
		// Saved first, so the stock that comes back names it (REC-2).
		id, err := repo.Insert(ctx, rec)
		if err != nil {
			return apperr.Internal("failed to record recovery", err)
		}

		switch in.Outcome {
		case OutcomeReplacement:
			// Back where intake would put it: the lot picked (a top-up blends its
			// cost) or a new lot with its expiry, each unit worth what was lost
			// plus the extra, so the stock carries the loss + the extra.
			cost := decimal.Zero
			if src.Quantity.IsPositive() {
				cost = src.LossValue.Div(src.Quantity).Round(4)
			}
			cost = cost.Add(extra)
			stk := stock.NewRepository(tx)
			if _, err := tx.ExecContext(ctx, `SELECT 1 FROM stock WHERE product_id = $1 FOR UPDATE`, src.ProductID); err != nil {
				return apperr.Internal("failed to hold stock", err)
			}
			var before *decimal.Decimal
			if in.BatchID > 0 {
				var c decimal.Decimal
				if err := tx.GetContext(ctx, &c, `SELECT cost_price FROM stock_batches WHERE id = $1 AND product_id = $2`, in.BatchID, src.ProductID); err != nil {
					return apperr.Validation("that batch is not this product's")
				}
				before = &c
			}
			refType := "recovery"
			lot, err := stk.AddStock(ctx, stock.AddStockInput{ProductID: src.ProductID, BatchID: in.BatchID, Qty: src.Quantity.String(),
				Cost: cost.String(), Price: in.Price, Expiry: in.Expiry, Note: "supplier replacement received", Source: "recovery",
				Move: stock.MoveRecovery, RefType: &refType, RefID: &id, Blend: true}, userID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE loss_recoveries SET lot_id = $2, lot_cost_before = $3 WHERE id = $1`, id, lot, before); err != nil {
				return apperr.Internal("failed to record recovery", err)
			}
			if rec.ExtraCharge.IsPositive() {
				sup, err := lockSupplier(ctx, tx, *in.SupplierID)
				if err != nil {
					return err
				}
				if !sup.IsActive {
					return apperr.Conflict("this supplier is disabled — bring them back first")
				}
				if err := suppliers.NewRepository(tx).AddBalance(ctx, sup.ID, rec.ExtraCharge); err != nil {
					return apperr.Internal("failed to charge supplier", err)
				}
			}

		case OutcomePaid:
			sup, err := lockSupplier(ctx, tx, *in.SupplierID)
			if err != nil {
				return err
			}
			if err := suppliers.NewRepository(tx).AddBalance(ctx, sup.ID, paidAmount.Neg()); err != nil {
				return apperr.Internal("failed to credit supplier", err)
			}
			if err := creditBills(ctx, tx, id, sup, paidAmount); err != nil {
				return err
			}
		}
		return nil
	})
}

// lockSupplier holds the supplier (last, after any stock), as payments do.
func lockSupplier(ctx context.Context, tx *sqlx.Tx, id int64) (*suppliers.Supplier, error) {
	sup, err := suppliers.NewRepository(tx).LockByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("supplier")
		}
		return nil, apperr.Internal("failed to load the supplier", err)
	}
	return sup, nil
}

// creditBills puts a supplier's credit (already off their balance, which took
// the old debt down with it) on their open bills, oldest first, and keeps where
// it went: each bill, the old debt, and what's left for their next delivery —
// only what they really owe us now (REC-3).
func creditBills(ctx context.Context, tx *sqlx.Tx, recoveryID int64, before *suppliers.Supplier, amount decimal.Decimal) error {
	bills := purchases.NewRepository(tx)
	open, err := bills.OpenBySupplier(ctx, before.ID)
	if err != nil {
		return apperr.Internal("failed to load open bills", err)
	}
	left := amount
	for _, pu := range open {
		if !left.IsPositive() {
			break
		}
		applied, err := bills.ApplyCredit(ctx, pu.ID, left)
		if err != nil {
			return apperr.Internal("failed to credit a bill", err)
		}
		if applied.IsPositive() {
			if _, err := tx.ExecContext(ctx, `INSERT INTO loss_recovery_bills (recovery_id, purchase_id, amount) VALUES ($1, $2, $3)`,
				recoveryID, pu.ID, applied); err != nil {
				return apperr.Internal("failed to keep a bill's credit", err)
			}
		}
		left = left.Sub(applied)
	}
	after, err := suppliers.NewRepository(tx).FindByID(ctx, before.ID)
	if err != nil {
		return apperr.Internal("failed to load the supplier", err)
	}
	unapplied := decimal.Max(decimal.Min(left, after.OutstandingBalance.Neg()), decimal.Zero)
	if _, err := tx.ExecContext(ctx, `UPDATE loss_recoveries SET credit_unapplied = $2, credit_waited = $2, old_debt = $3 WHERE id = $1`,
		recoveryID, unapplied, before.OpeningUnlinked.Sub(after.OpeningUnlinked)); err != nil {
		return apperr.Internal("failed to record recovery", err)
	}
	return nil
}

// LossProduct is the product a loss is of.
func (s *Service) LossProduct(ctx context.Context, sourceType string, sourceID int64) (int64, error) {
	var (
		src *source
		err error
	)
	if sourceType == SourceWarranty {
		src, err = s.repo.warrantySource(ctx, sourceID)
	} else {
		src, err = s.repo.damageSource(ctx, sourceID)
	}
	if err != nil {
		return 0, err
	}
	return src.ProductID, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
