package conversions

import (
	"context"
	"database/sql"
	"errors"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/stock"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Run converts fromQty units of the source product into toQty units of the
// destination (zero toQty = fromQty × ratio), moving the value across in one
// transaction.
//
// intoStock matters only when the destination's stock is frozen and the
// source's isn't: false = the pieces were put out for sale (not counted while
// frozen), true = they went into storage and are added to stock.
func (s *Service) Run(ctx context.Context, conversionID int64, fromQty, toQty decimal.Decimal, userID int64, intoStock bool) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		return runTx(ctx, tx, conversionID, fromQty, toQty, userID, intoStock)
	})
}

// RunTx is Run inside the caller's transaction (the loose-version endpoint runs
// the first conversion in the same tx that makes the product).
//
// toQty is what was really weighed out: a 25 kg bag often gives 24.6. The bags'
// cost is spread over that, so the loss shows in the loose cost instead of
// vanishing. Each bag opened becomes its own loose batch with that bag's expiry,
// batch no and cost, so FEFO and expiry prompts still see it.
func RunTx(ctx context.Context, tx *sqlx.Tx, conversionID int64, fromQty, toQty decimal.Decimal, userID int64) error {
	return runTx(ctx, tx, conversionID, fromQty, toQty, userID, false)
}

func runTx(ctx context.Context, tx *sqlx.Tx, conversionID int64, fromQty, toQty decimal.Decimal, userID int64, intoStock bool) error {
	if !fromQty.IsPositive() {
		return apperr.Validation("quantity must be greater than zero")
	}
	if toQty.IsNegative() {
		return apperr.Validation("the amount you got can't be negative")
	}
	repo := NewRepository(tx)
	stk := stock.NewRepository(tx)

	cv, err := repo.FindByID(ctx, conversionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("conversion")
		}
		return apperr.Internal("failed to load conversion", err)
	}
	if toQty.IsZero() {
		toQty = fromQty.Mul(cv.Ratio)
	}
	toQty = toQty.Round(3)
	if !toQty.IsPositive() {
		return apperr.Validation("the amount you got must be greater than zero")
	}
	if err := stk.CheckWhole(ctx, cv.FromProductID, fromQty); err != nil {
		return err
	}
	if err := stk.CheckWhole(ctx, cv.ToProductID, toQty); err != nil {
		return err
	}

	// Deplete the source product.
	ok, err := stk.DecrementGuarded(ctx, cv.FromProductID, fromQty)
	if err != nil {
		return apperr.Internal("failed to update source stock", err)
	}
	if !ok {
		return apperr.Conflict("not enough stock of the source product")
	}
	lots, _, err := stk.DepleteFEFOLots(ctx, cv.FromProductID, fromQty)
	if err != nil {
		return apperr.Internal("failed to deplete source batches", err)
	}
	refOut := "conversion"
	cid := conversionID
	if err := stk.InsertMovement(ctx, stock.MovementInput{
		ProductID: cv.FromProductID, Type: stock.MoveConversion, Quantity: fromQty.Neg(),
		ReferenceID: &cid, ReferenceType: &refOut, UserID: userID,
	}); err != nil {
		return apperr.Internal("failed to record source movement", err)
	}

	// A frozen destination's pieces put out for sale aren't counted (they sell
	// without touching stock); its movement records them with quantity 0.
	frozen, err := stk.Frozen(ctx, cv.ToProductID)
	if err != nil {
		return apperr.Internal("failed to read destination stock", err)
	}
	// Bag and loose both frozen: nothing is asked, the pieces go out for sale.
	if frozen && intoStock {
		if intoStock, err = stk.Frozen(ctx, cv.FromProductID); err != nil {
			return apperr.Internal("failed to read source stock", err)
		}
		intoStock = !intoStock
	}
	inQty := toQty
	var note *string
	if frozen && !intoStock {
		inQty = decimal.Zero
		n := toQty.String() + " put out for sale (frozen, not counted)"
		note = &n
	} else {
		// One destination batch per bag opened, carrying that bag's value.
		if err := stk.Increment(ctx, cv.ToProductID, toQty); err != nil {
			return apperr.Internal("failed to update destination stock", err)
		}
		for _, b := range splitYield(lots, fromQty, toQty) {
			b.ProductID = cv.ToProductID
			if _, err := stk.InsertBatch(ctx, b); err != nil {
				return apperr.Internal("failed to open destination batch", err)
			}
		}
	}
	if err := stk.InsertMovement(ctx, stock.MovementInput{
		ProductID: cv.ToProductID, Type: stock.MoveConversion, Quantity: inQty,
		ReferenceID: &cid, ReferenceType: &refOut, UserID: userID, Note: note,
	}); err != nil {
		return apperr.Internal("failed to record destination movement", err)
	}
	return repo.LogRun(ctx, conversionID, cv.FromProductID, cv.ToProductID, userID, fromQty, toQty, inQty.IsZero())
}

// splitYield shares toQty among the consumed lots by what each gave
// (take ÷ fromQty × toQty, 3 dp; the last takes the remainder so the total is
// exact). Each share's cost is its lot's value over the share, so value moves
// across exactly (± 0.01 a batch). A lot whose share rounds to nothing folds
// into the next. No lots at all (stock with no batches) → one batch at cost 0,
// which InsertBatch prices at the loose product's own cost, as before.
func splitYield(lots []stock.ConsumedLot, fromQty, toQty decimal.Decimal) []stock.NewBatch {
	if len(lots) == 0 {
		return []stock.NewBatch{{Quantity: toQty, Source: "conversion"}}
	}
	out := make([]stock.NewBatch, 0, len(lots))
	left := toQty
	carry := decimal.Zero // value of lots whose share rounded to zero
	for i, l := range lots {
		share := left
		if i < len(lots)-1 {
			share = l.Qty.Div(fromQty).Mul(toQty).Round(3)
		}
		value := l.Cost.Mul(l.Qty).Add(carry)
		if !share.IsPositive() {
			carry = value
			continue
		}
		carry = decimal.Zero
		left = left.Sub(share)
		out = append(out, stock.NewBatch{
			Quantity: share, CostPrice: value.Div(share).Round(4),
			ExpiryDate: l.Expiry, BatchNo: l.BatchNo, Source: "conversion",
		})
	}
	// The earlier shares rounded up to the whole yield, leaving the last lot(s)
	// nothing: their value goes on the last batch made, never lost (STK-11).
	if n := len(out); carry.IsPositive() && n > 0 {
		b := &out[n-1]
		b.CostPrice = b.CostPrice.Mul(b.Quantity).Add(carry).Div(b.Quantity).Round(4)
	}
	return out
}
