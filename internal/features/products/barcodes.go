package products

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// BarcodeRow is one of a product's codes: the main one (products.barcode) or an
// extra (product_barcodes), with when it was added and last scanned.
type BarcodeRow struct {
	Barcode       string     `db:"barcode"`
	Main          bool       `db:"main"`
	Source        string     `db:"source"`
	AddedAt       *time.Time `db:"added_at"`
	LastScannedAt *time.Time `db:"last_scanned_at"`
}

// barcodeSources are where an extra barcode can come from.
var barcodeSources = map[string]bool{"intake": true, "receive": true, "form": true, "merge": true, "import": true}

// Barcodes lists a product's codes, the main one first, then extras oldest first.
func (s *Service) Barcodes(ctx context.Context, productID int64) ([]BarcodeRow, error) {
	rows := []BarcodeRow{}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT barcode, true AS main, '' AS source, created_at AS added_at, barcode_scanned_at AS last_scanned_at
		FROM products WHERE id = $1 AND barcode IS NOT NULL
		UNION ALL
		SELECT barcode, false, source, added_at, last_scanned_at
		FROM product_barcodes WHERE product_id = $1 AND pack_id IS NULL
		ORDER BY main DESC, added_at`, productID)
	return rows, err
}

// barcodeOwner names the product carrying a code (main or extra), "" when free.
func barcodeOwner(ctx context.Context, q sqlx.QueryerContext, code string) (int64, string, error) {
	var o struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	err := sqlx.GetContext(ctx, q, &o, `SELECT p.id, p.name FROM products p WHERE `+barcodeIs("$1")+` LIMIT 1`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", nil
	}
	return o.ID, o.Name, err
}

// AddBarcode gives a product an extra code. A code another product carries is
// refused naming that product; one this product already has is a no-op.
func (s *Service) AddBarcode(ctx context.Context, productID int64, code, source string) error {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 50 {
		return apperr.Validation("enter a barcode (up to 50 characters)")
	}
	if !barcodeSources[source] {
		return apperr.Validation("unknown barcode source")
	}
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		id, name, err := barcodeOwner(ctx, tx, code)
		if err != nil {
			return apperr.Internal("failed to check barcode", err)
		}
		if id == productID {
			return nil
		}
		if id != 0 {
			return apperr.Conflict("that barcode is already used by " + name)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, $2, $3)`,
			productID, code, source); err != nil {
			return apperr.Internal("failed to add barcode", err)
		}
		return nil
	})
}

// RemoveBarcode drops one of a product's extra codes. The main barcode is
// changed through the product form, never removed here.
func (s *Service) RemoveBarcode(ctx context.Context, productID int64, code string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM product_barcodes WHERE product_id = $1 AND barcode = $2 AND pack_id IS NULL`, productID, strings.TrimSpace(code))
	if err != nil {
		return apperr.Internal("failed to remove barcode", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("extra barcode")
	}
	return nil
}

// MakeMainBarcode promotes an extra to the main barcode (labels print it). The
// old main stays on the product as an extra, keeping its last-scan date.
func (s *Service) MakeMainBarcode(ctx context.Context, productID int64, code string) error {
	code = strings.TrimSpace(code)
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		var extra struct {
			LastScannedAt *time.Time `db:"last_scanned_at"`
		}
		err := tx.GetContext(ctx, &extra, `DELETE FROM product_barcodes WHERE product_id = $1 AND barcode = $2 AND pack_id IS NULL RETURNING last_scanned_at`, productID, code)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound("extra barcode")
		}
		if err != nil {
			return apperr.Internal("failed to change the main barcode", err)
		}
		var old struct {
			Barcode   *string    `db:"barcode"`
			ScannedAt *time.Time `db:"barcode_scanned_at"`
		}
		if err := tx.GetContext(ctx, &old, `SELECT barcode, barcode_scanned_at FROM products WHERE id = $1 FOR UPDATE`, productID); err != nil {
			return apperr.Internal("failed to read product", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET barcode = $2, barcode_scanned_at = $3 WHERE id = $1`,
			productID, code, extra.LastScannedAt); err != nil {
			return apperr.Internal("failed to change the main barcode", err)
		}
		if old.Barcode != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source, last_scanned_at) VALUES ($1, $2, 'form', $3)`,
				productID, *old.Barcode, old.ScannedAt); err != nil {
				return apperr.Internal("failed to keep the old barcode", err)
			}
		}
		return nil
	})
}
