package conversions

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/products"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// LooseInput makes a packed product's loose version: a 25 kg bag of rice sold
// by the kg. Ratio is how many loose units one source unit gives; ConvertNow
// (source units, zero = none) opens that many straight away.
type LooseInput struct {
	SourceID     int64
	Name         string
	UnitID       int64
	Ratio        string
	SellingPrice string
	Barcode      string
	ConvertNow   decimal.Decimal
	UserID       int64
}

// MakeLoose creates the loose product (copying the source's category, supplier,
// tax, reorder level, warranty and local name), the conversion from the source,
// and the first run when asked — in one transaction, so a refusal anywhere
// (a unit clash, a bad ratio, not enough bags) leaves nothing behind.
func MakeLoose(ctx context.Context, db *sqlx.DB, in LooseInput) (productID, conversionID int64, err error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return 0, 0, apperr.Validation("give the loose version a name")
	}
	ratio, perr := money.Parse(in.Ratio)
	if perr != nil || !ratio.IsPositive() {
		return 0, 0, apperr.Validation("how many make one must be greater than zero")
	}
	if in.ConvertNow.IsNegative() {
		return 0, 0, apperr.Validation("convert now can't be negative")
	}
	err = appdb.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		src, err := products.NewRepository(tx).FindByID(ctx, in.SourceID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("product")
			}
			return apperr.Internal("failed to load product", err)
		}
		switch {
		case src.IsService:
			return apperr.Validation("a service has no stock to sell loose")
		case !src.IsActive:
			return apperr.Validation("enable the product first")
		case src.UnitID == in.UnitID:
			return apperr.Validation("the loose version needs a different unit from " + src.Name)
		}
		var barcode *string
		if b := strings.TrimSpace(in.Barcode); b != "" {
			barcode = &b
		}
		productID, err = products.CreateTx(ctx, tx, products.CreateInput{
			Name: name, NameLocal: src.NameLocal, Barcode: barcode, CategoryID: src.CategoryID, UnitID: in.UnitID,
			CostPrice: src.CostPrice.Div(ratio).Round(2).String(), SellingPrice: orZero(in.SellingPrice),
			WholesalePrice: "0", TaxRate: src.TaxRate.String(), ReorderLevel: src.ReorderLevel,
			WarrantyMonths: src.WarrantyMonths, PreferredSupplierID: src.PreferredSupplierID,
		})
		if err != nil {
			return err
		}
		conversionID, err = NewRepository(tx).Create(ctx, CreateInput{FromProductID: src.ID, ToProductID: productID}, ratio)
		if err != nil {
			return apperr.Internal("failed to save the conversion", err)
		}
		if in.ConvertNow.IsPositive() {
			return RunTx(ctx, tx, conversionID, in.ConvertNow, decimal.Zero, in.UserID)
		}
		return nil
	})
	return productID, conversionID, err
}

func orZero(s string) string {
	if s = strings.TrimSpace(s); s == "" {
		return "0"
	}
	return s
}
