package conversions

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Fold turns a card + loose twin (a conversion card → loose) into one product
// sold by the pack: every card on the shelf is opened into the loose product
// (batches, expiry and cost carried, as a run does), the loose product gets a
// pack of the ratio's size at the card's prices, the card's barcodes move onto
// that pack, and the conversion and the card are turned off. Their history
// stays as it was (those sales were cards). One transaction; returns the pack.
func Fold(ctx context.Context, db *sqlx.DB, conversionID int64, packName string, userID int64) (int64, error) {
	name := strings.TrimSpace(packName)
	if name == "" || len([]rune(name)) > 30 {
		return 0, apperr.Validation("give the pack a name (up to 30 characters), e.g. Card")
	}
	var packID int64
	err := appdb.WithTx(ctx, db, func(tx *sqlx.Tx) (err error) {
		packID, err = FoldTx(ctx, tx, conversionID, name, userID)
		return err
	})
	return packID, err
}

// FoldTx is Fold inside the caller's transaction.
func FoldTx(ctx context.Context, tx *sqlx.Tx, conversionID int64, name string, userID int64) (int64, error) {
	var packID int64
	err := func() error {
		cv, err := NewRepository(tx).FindByID(ctx, conversionID)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("conversion")
		}
		if err != nil {
			return apperr.Internal("failed to load conversion", err)
		}
		if !cv.IsActive {
			return apperr.Validation("that conversion is turned off")
		}
		if cv.Ratio.LessThanOrEqual(decimal.NewFromInt(1)) || !cv.Ratio.Equal(cv.Ratio.Round(3)) {
			return apperr.Validation("a pack must hold more than one " + cv.ToUnitAbbr)
		}
		var to struct {
			AllowDecimal bool `db:"allow_decimal"`
			IsService    bool `db:"is_service"`
		}
		if err := tx.GetContext(ctx, &to, `SELECT u.allow_decimal, p.is_service FROM products p JOIN units u ON u.id = p.unit_id WHERE p.id = $1`, cv.ToProductID); err != nil {
			return apperr.Internal("failed to load product", err)
		}
		if to.AllowDecimal || to.IsService {
			return apperr.Validation(cv.ToName + " is not counted in whole units, so it can't be sold by the pack")
		}
		if cv.FromStock.IsNegative() {
			return apperr.Validation(cv.FromName + " shows less than nothing in stock — count it first")
		}
		if cv.FromStock.IsPositive() {
			if err := RunTx(ctx, tx, conversionID, cv.FromStock, decimal.Zero, userID); err != nil {
				return err
			}
		}
		var from struct {
			Barcode   *string         `db:"barcode"`
			Selling   decimal.Decimal `db:"selling_price"`
			Wholesale decimal.Decimal `db:"wholesale_price"`
		}
		if err := tx.GetContext(ctx, &from, `SELECT barcode, selling_price, wholesale_price FROM products WHERE id = $1 FOR UPDATE`, cv.FromProductID); err != nil {
			return apperr.Internal("failed to load product", err)
		}
		err = tx.GetContext(ctx, &packID, `
			INSERT INTO product_packs (product_id, name, size, selling_price, wholesale_price)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, cv.ToProductID, name, cv.Ratio, from.Selling, from.Wholesale)
		if err != nil {
			if appdb.IsUniqueViolation(err) {
				return apperr.Conflict(cv.ToName + " already has a pack called " + name)
			}
			return apperr.Internal("failed to save the pack", err)
		}
		// The card's codes now scan to a card of the loose product.
		if _, err := tx.ExecContext(ctx, `UPDATE products SET barcode = NULL, is_active = false WHERE id = $1`, cv.FromProductID); err != nil {
			return apperr.Internal("failed to retire the card", err)
		}
		if from.Barcode != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source, pack_id) VALUES ($1, $2, 'pack', $3)`,
				cv.ToProductID, *from.Barcode, packID); err != nil {
				return apperr.Internal("failed to move the card's barcode", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE product_barcodes SET product_id = $2, pack_id = $3, source = 'pack' WHERE product_id = $1`,
			cv.FromProductID, cv.ToProductID, packID); err != nil {
			return apperr.Internal("failed to move the card's barcodes", err)
		}
		if err := NewRepository(tx).Delete(ctx, conversionID); err != nil {
			return apperr.Internal("failed to turn the conversion off", err)
		}
		return nil
	}()
	return packID, err
}
