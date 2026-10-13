package recovery

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
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

// SourceInfo resolves the product name + worth of a loss for the recovery form.
func (s *Service) SourceInfo(ctx context.Context, sourceType string, sourceID int64) (*SourceInfo, error) {
	var (
		src *source
		err error
	)
	switch sourceType {
	case SourceWarranty:
		src, err = s.repo.warrantySource(ctx, sourceID)
	case SourceDamage:
		src, err = s.repo.damageSource(ctx, sourceID)
	default:
		return nil, apperr.BadRequest("invalid loss source")
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("loss")
		}
		return nil, apperr.Internal("failed to load loss", err)
	}
	return &SourceInfo{ProductName: src.ProductName, LossValue: src.LossValue}, nil
}

// DamageLosses backs the admin Damage report.
func (s *Service) DamageLosses(ctx context.Context) ([]DamageLoss, error) {
	rows, err := s.repo.DamageLosses(ctx, 200)
	if err != nil {
		return nil, apperr.Internal("failed to load damage losses", err)
	}
	return rows, nil
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
		stk := stock.NewRepository(tx)

		// Resolve the loss being recovered.
		var (
			src *source
			err error
		)
		switch in.SourceType {
		case SourceWarranty:
			src, err = repo.warrantySource(ctx, in.SourceID)
		default:
			src, err = repo.damageSource(ctx, in.SourceID)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("loss")
			}
			return apperr.Internal("failed to load loss", err)
		}
		if in.SourceType == SourceWarranty && src.Status != "replaced" {
			return apperr.Conflict("only a replaced unit can be recovered from the supplier")
		}

		// Don't recover the same loss twice.
		already, err := repo.RecoveredQtyForSource(ctx, in.SourceType, in.SourceID)
		if err != nil {
			return apperr.Internal("failed to check existing recovery", err)
		}
		if already.GreaterThanOrEqual(src.Quantity) {
			return apperr.Conflict("this loss has already been recovered")
		}

		rec := Recovery{
			SourceType: in.SourceType,
			SourceID:   in.SourceID,
			ProductID:  src.ProductID,
			SupplierID: in.SupplierID,
			Outcome:    in.Outcome,
			Quantity:   src.Quantity,
			LossValue:  src.LossValue,
			Note:       nilIfEmpty(in.Note),
			HandledBy:  userID,
		}

		switch in.Outcome {
		case OutcomeReplacement:
			// Supplier returned goods → back into inventory where intake would
			// put them: the batch picked (a top-up) or a new batch with its
			// expiry, valued at what was lost per unit so the recovered value
			// is the loss (the product's cost may have moved since).
			// Any extra the supplier asked on top is owed to them and is part
			// of what this stock cost.
			cost := decimal.Zero
			if src.Quantity.IsPositive() {
				cost = src.LossValue.Div(src.Quantity).Round(2)
			}
			cost = cost.Add(extra)
			refType := "recovery"
			if _, err := stk.AddStock(ctx, stock.AddStockInput{ProductID: src.ProductID, BatchID: in.BatchID, Qty: src.Quantity.String(),
				Cost: cost.String(), Price: in.Price, Expiry: in.Expiry, Note: "supplier replacement received", Source: "recovery",
				Move: stock.MoveRecovery, RefType: &refType}, userID); err != nil {
				return err
			}
			if extra.IsPositive() {
				rec.ExtraCharge = extra.Mul(src.Quantity).Round(2)
				if err := suppliers.NewRepository(tx).AddBalance(ctx, *in.SupplierID, rec.ExtraCharge); err != nil {
					return apperr.Internal("failed to charge supplier", err)
				}
			}
			// The goods came back, so the original loss is recovered in kind.
			rec.RecoveredValue = src.LossValue

		case OutcomePaid:
			sup := suppliers.NewRepository(tx)
			if err := sup.AddBalance(ctx, *in.SupplierID, paidAmount.Neg()); err != nil {
				return apperr.Internal("failed to credit supplier", err)
			}
			rec.RecoveredValue = paidAmount

		case OutcomeWrittenOff:
			rec.RecoveredValue = decimal.Zero
		}

		if _, err := repo.Insert(ctx, rec); err != nil {
			return apperr.Internal("failed to record recovery", err)
		}
		return nil
	})
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
