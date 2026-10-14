package products

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Pack is a way to sell, buy and scan a product by more than one unit at a time
// (Card ×12 of a tablet). Stock stays in units; see migrations/00005.
type Pack struct {
	ID             int64           `db:"id"`
	ProductID      int64           `db:"product_id"`
	Name           string          `db:"name"`
	Size           decimal.Decimal `db:"size"`
	SellingPrice   decimal.Decimal `db:"selling_price"`   // 0 = size × the unit price
	WholesalePrice decimal.Decimal `db:"wholesale_price"` // 0 = the selling price
	SetAtTill      bool            `db:"set_at_till"`
	Barcodes       pq.StringArray  `db:"barcodes"`
}

// PackInput creates (ID 0) or edits a pack. Barcode is one code to add ("" = none).
type PackInput struct {
	ID             int64
	Name           string
	Size           string
	SellingPrice   string
	WholesalePrice string
	Barcode        string
	SetAtTill      bool
}

const packSelect = `
	SELECT k.id, k.product_id, k.name, k.size, k.selling_price, k.wholesale_price, k.set_at_till,
	       ARRAY(SELECT b.barcode FROM product_barcodes b WHERE b.pack_id = k.id ORDER BY b.id) AS barcodes
	FROM product_packs k`

// Packs lists a product's packs, smallest first.
func (s *Service) Packs(ctx context.Context, productID int64) ([]Pack, error) {
	out := []Pack{}
	err := s.db.SelectContext(ctx, &out, packSelect+` WHERE k.product_id = $1 ORDER BY k.size, k.id`, productID)
	return out, err
}

// PacksFor loads the packs of many products in one query (till pages, a sale).
func PacksFor(ctx context.Context, q sqlx.QueryerContext, ids []int64) (map[int64][]Pack, error) {
	out := map[int64][]Pack{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []Pack
	if err := sqlx.SelectContext(ctx, q, &rows, packSelect+` WHERE k.product_id = ANY($1) ORDER BY k.size, k.id`, pq.Array(ids)); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ProductID] = append(out[r.ProductID], r)
	}
	return out, nil
}

// PackForBarcode resolves a code to the pack carrying it (0, 0 when it is no pack's).
func PackForBarcode(ctx context.Context, q sqlx.QueryerContext, code string) (productID, packID int64, err error) {
	var r struct {
		ProductID int64 `db:"product_id"`
		PackID    int64 `db:"pack_id"`
	}
	err = sqlx.GetContext(ctx, q, &r, `SELECT product_id, pack_id FROM product_barcodes WHERE barcode = $1 AND pack_id IS NOT NULL`, strings.TrimSpace(code))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return r.ProductID, r.PackID, err
}

// SavePack creates or edits one of a product's packs (whole-unit, stocked
// products only) and returns it.
func (s *Service) SavePack(ctx context.Context, productID int64, in PackInput) (*Pack, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 30 {
		return nil, apperr.Validation("give the pack a name (up to 30 characters), e.g. Card")
	}
	size, err := money.Parse(in.Size)
	if err != nil || size.LessThanOrEqual(decimal.NewFromInt(1)) || !size.Equal(size.Round(3)) {
		return nil, apperr.Validation("how many in one " + name + "? It must be more than 1")
	}
	sell, err := parsePrice(in.SellingPrice)
	if err != nil {
		return nil, apperr.Validation("the pack price is not a valid amount")
	}
	whole, err := parsePrice(in.WholesalePrice)
	if err != nil {
		return nil, apperr.Validation("the pack wholesale price is not a valid amount")
	}
	code := strings.TrimSpace(in.Barcode)
	if len(code) > 50 {
		return nil, apperr.Validation("a barcode is up to 50 characters")
	}
	var id int64
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		var p struct {
			IsService    bool `db:"is_service"`
			AllowDecimal bool `db:"allow_decimal"`
		}
		err := tx.GetContext(ctx, &p, `SELECT p.is_service, u.allow_decimal FROM products p JOIN units u ON u.id = p.unit_id WHERE p.id = $1 FOR UPDATE OF p`, productID)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("product")
		}
		if err != nil {
			return apperr.Internal("failed to read product", err)
		}
		if p.IsService || p.AllowDecimal {
			return apperr.Validation("packs are for items counted in whole units")
		}
		if in.ID == 0 {
			err = tx.GetContext(ctx, &id, `
				INSERT INTO product_packs (product_id, name, size, selling_price, wholesale_price, set_at_till)
				VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, productID, name, size, sell, whole, in.SetAtTill)
		} else {
			id = in.ID
			var res sql.Result
			res, err = tx.ExecContext(ctx, `
				UPDATE product_packs SET name = $3, size = $4, selling_price = $5, wholesale_price = $6, set_at_till = false
				WHERE id = $1 AND product_id = $2`, in.ID, productID, name, size, sell, whole)
			if err == nil {
				if n, _ := res.RowsAffected(); n == 0 {
					return apperr.NotFound("pack")
				}
			}
		}
		if err != nil {
			if appdb.IsUniqueViolation(err) {
				return apperr.Conflict("this item already has a pack called " + name)
			}
			return apperr.Internal("failed to save pack", err)
		}
		if code == "" {
			return nil
		}
		owner, ownerName, err := barcodeOwner(ctx, tx, code)
		if err != nil {
			return apperr.Internal("failed to check barcode", err)
		}
		if owner != 0 && owner != productID {
			return apperr.Conflict("that barcode is already used by " + ownerName)
		}
		var onPack int64
		_ = tx.GetContext(ctx, &onPack, `SELECT COALESCE(pack_id, 0) FROM product_barcodes WHERE barcode = $1`, code)
		if onPack == id {
			return nil
		}
		if onPack != 0 {
			return apperr.Conflict("that barcode is already on another pack of this item")
		}
		// This product's own code (main or extra) is really the card's: it moves
		// onto the pack, so scanning it sells a card from now on.
		if _, err := tx.ExecContext(ctx, `UPDATE products SET barcode = NULL WHERE id = $1 AND barcode = $2`, productID, code); err != nil {
			return apperr.Internal("failed to move barcode", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM product_barcodes WHERE barcode = $1 AND product_id = $2 AND pack_id IS NULL`, code, productID); err != nil {
			return apperr.Internal("failed to move barcode", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source, pack_id) VALUES ($1, $2, 'pack', $3)`,
			productID, code, id); err != nil {
			return apperr.Internal("failed to add barcode", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out Pack
	if err := s.db.GetContext(ctx, &out, packSelect+` WHERE k.id = $1`, id); err != nil {
		return nil, apperr.Internal("failed to read pack", err)
	}
	return &out, nil
}

func parsePrice(s string) (decimal.Decimal, error) {
	if strings.TrimSpace(s) == "" {
		return decimal.Zero, nil
	}
	d, err := money.Parse(s)
	if err != nil || d.IsNegative() {
		return decimal.Zero, errors.New("bad price")
	}
	return d, nil
}

// DeletePack removes a pack and its barcodes. Past sales keep their label.
func (s *Service) DeletePack(ctx context.Context, productID, packID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM product_packs WHERE id = $1 AND product_id = $2`, packID, productID)
	if err != nil {
		return apperr.Internal("failed to remove pack", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("pack")
	}
	return nil
}

// RemovePackBarcode drops one code from a pack.
func (s *Service) RemovePackBarcode(ctx context.Context, productID, packID int64, code string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM product_barcodes WHERE product_id = $1 AND pack_id = $2 AND barcode = $3`,
		productID, packID, strings.TrimSpace(code))
	if err != nil {
		return apperr.Internal("failed to remove barcode", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("pack barcode")
	}
	return nil
}

// ClearPacksSetAtTill marks a product's till-made pack sizes as checked.
func (s *Service) ClearPacksSetAtTill(ctx context.Context, productID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE product_packs SET set_at_till = false WHERE product_id = $1 AND set_at_till`, productID)
	return err
}
