package products

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Merge folds a duplicate product (from) into the one the shop keeps (into): its
// barcodes, stock and lots, sales / purchases / returns history and set-up rows
// move over, plugins move their own rows, and the duplicate is retired (hidden,
// barcode cleared, merged_into_id set). One transaction; not undoable.

// MergeHook, when non-nil, lets plugins move their own rows inside the merge's
// transaction (set by the web layer from plugin.ProductMergers). nil = core only.
var MergeHook func(ctx context.Context, tx *sqlx.Tx, fromID, intoID int64) error

// MergePreview is what the confirm screen shows. Refusal, when set, is why the
// merge can't run (and nothing else matters).
type MergePreview struct {
	FromName   string
	IntoName   string
	Barcodes   int             // the duplicate's main + extras
	Stock      decimal.Decimal // on hand
	Lots       int             // live lots
	Sales      int             // sales with a line of it
	Purchases  int             // purchases with a line of it
	PriceClash bool            // prices differ and some of its stock follows its own price
	FromPrice  decimal.Decimal
	IntoPrice  decimal.Decimal
	Refusal    string
}

type mergeSide struct {
	ID        int64           `db:"id"`
	Name      string          `db:"name"`
	UnitID    int64           `db:"unit_id"`
	IsService bool            `db:"is_service"`
	IsActive  bool            `db:"is_active"`
	Merged    *int64          `db:"merged_into_id"`
	Price     decimal.Decimal `db:"selling_price"`
}

func loadSide(ctx context.Context, q sqlx.QueryerContext, id int64, lock bool) (*mergeSide, error) {
	var s mergeSide
	sel := `SELECT id, name, unit_id, is_service, is_active, merged_into_id, selling_price FROM products WHERE id = $1`
	if lock {
		sel += ` FOR UPDATE`
	}
	if err := sqlx.GetContext(ctx, q, &s, sel, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("product")
		}
		return nil, err
	}
	return &s, nil
}

// heldIn names who holds a bill with the product in it ("" when none). Held carts
// are JSON: a karots-pos list of lines ({"id"}) or {"lines":[{"productId"}]}, with
// recipe components inside lines.
func heldIn(ctx context.Context, q sqlx.QueryerContext, id int64) (string, error) {
	var who string
	err := sqlx.GetContext(ctx, q, &who, `
		SELECT COALESCE(u.name, 'a cashier') || ' at ' || to_char(h.created_at, 'HH24:MI')
		FROM held_sales h LEFT JOIN users u ON u.id = h.cashier_id
		WHERE jsonb_path_exists(h.cart, '$.** ? (@.productId == $id || @.product_id == $id || @.id == $id)', jsonb_build_object('id', $1::bigint))
		ORDER BY h.created_at LIMIT 1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return who, err
}

// refusal says why from can't be merged into into, "" when it can.
func refusal(ctx context.Context, q sqlx.QueryerContext, from, into *mergeSide) (string, error) {
	switch {
	case from.ID == into.ID:
		return "a product can't be merged into itself", nil
	case from.Merged != nil:
		return from.Name + " was already merged", nil
	case into.Merged != nil || !into.IsActive:
		return "keep an active product: " + into.Name + " is disabled", nil
	case from.IsService != into.IsService:
		return "a service can't be merged with a stock item", nil
	case from.UnitID != into.UnitID:
		return "the two are sold in different units — change one first", nil
	}
	who, err := heldIn(ctx, q, from.ID)
	if err != nil || who == "" {
		return "", err
	}
	return fmt.Sprintf("%s is in a bill held by %s — resume or clear it first", from.Name, who), nil
}

// MergePreview counts what a merge would move, or says why it can't run.
func (s *Service) MergePreview(ctx context.Context, fromID, intoID int64) (MergePreview, error) {
	from, err := loadSide(ctx, s.db, fromID, false)
	if err != nil {
		return MergePreview{}, err
	}
	into, err := loadSide(ctx, s.db, intoID, false)
	if err != nil {
		return MergePreview{}, err
	}
	pv := MergePreview{FromName: from.Name, IntoName: into.Name, FromPrice: from.Price, IntoPrice: into.Price}
	if pv.Refusal, err = refusal(ctx, s.db, from, into); err != nil || pv.Refusal != "" {
		return pv, err
	}
	var c struct {
		Barcodes  int             `db:"barcodes"`
		Stock     decimal.Decimal `db:"stock"`
		Lots      int             `db:"lots"`
		Following int             `db:"following"`
		Sales     int             `db:"sales"`
		Purchases int             `db:"purchases"`
	}
	if err := s.db.GetContext(ctx, &c, `SELECT
		(SELECT COUNT(*) FROM product_barcodes WHERE product_id = $1) + (SELECT COUNT(barcode) FROM products WHERE id = $1) AS barcodes,
		COALESCE((SELECT quantity FROM stock WHERE product_id = $1), 0) AS stock,
		(SELECT COUNT(*) FROM stock_batches WHERE product_id = $1 AND qty_remaining > 0) AS lots,
		(SELECT COUNT(*) FROM stock_batches WHERE product_id = $1 AND qty_remaining > 0 AND selling_price = 0) AS following,
		(SELECT COUNT(DISTINCT sale_id) FROM sale_items WHERE product_id = $1) AS sales,
		(SELECT COUNT(DISTINCT purchase_id) FROM purchase_items WHERE product_id = $1) AS purchases`, fromID); err != nil {
		return pv, err
	}
	pv.Barcodes, pv.Stock, pv.Lots, pv.Sales, pv.Purchases = c.Barcodes, c.Stock, c.Lots, c.Sales, c.Purchases
	pv.PriceClash = !from.Price.Equal(into.Price) && c.Following > 0
	return pv, nil
}

// Merge folds fromID into intoID. lotPrice "keep" stamps the duplicate's price on
// its lots that followed it (so they sell as before); anything else lets them
// follow the kept product's price.
func (s *Service) Merge(ctx context.Context, fromID, intoID int64, lotPrice string) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		// Lock both rows, lower id first, so two merges can't deadlock or cross.
		lo, hi := min(fromID, intoID), max(fromID, intoID)
		a, err := loadSide(ctx, tx, lo, true)
		if err != nil {
			return err
		}
		b, err := loadSide(ctx, tx, hi, true)
		if err != nil {
			return err
		}
		from, into := a, b
		if fromID != lo {
			from, into = b, a
		}
		if why, err := refusal(ctx, tx, from, into); err != nil {
			return err
		} else if why != "" {
			return apperr.Validation(why)
		}
		if lotPrice == "keep" && !from.Price.Equal(into.Price) {
			if _, err := tx.ExecContext(ctx, `UPDATE stock_batches SET selling_price = $2
				WHERE product_id = $1 AND selling_price = 0 AND qty_remaining > 0`, fromID, from.Price); err != nil {
				return apperr.Internal("failed to keep the lot price", err)
			}
		}
		if err := mergeTables(ctx, tx, fromID, intoID); err != nil {
			return apperr.Internal("failed to merge", err)
		}
		if MergeHook != nil {
			if err := MergeHook(ctx, tx, fromID, intoID); err != nil {
				return err
			}
		}
		return nil
	})
}
