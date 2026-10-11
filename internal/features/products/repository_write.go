// Package products is the master catalog: CRUD plus barcode/search lookups used
// by both the admin panel and the cashier terminal.
package products

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lib/pq"

	"github.com/shopspring/decimal"
)

// SyncRow is the lean, read-only projection served to the stock_capture app's
// LAN catalog sync. category is the full "Parent > Child" path (matching the
// CSV import format) so a synced item's category aligns with export/import.
type SyncRow struct {
	ID           int64           `db:"id"            json:"id"`
	Name         string          `db:"name"          json:"name"`
	NameLocal    *string         `db:"name_local"    json:"name_local,omitempty"`
	Barcode      *string         `db:"barcode"       json:"barcode,omitempty"`
	Category     string          `db:"category"      json:"category"`
	Unit         string          `db:"unit"          json:"unit"`
	SellingPrice decimal.Decimal `db:"selling_price" json:"selling_price"`
	StockQty     decimal.Decimal `db:"stock_qty"     json:"stock_qty"`
	IsService    bool            `db:"is_service"    json:"is_service"`
}

// SyncCatalog returns every active product with its full category path, current
// on-hand quantity and current selling price. Read-only; no pagination — the app
// wants one atomic snapshot per sync.
func (r *Repository) SyncCatalog(ctx context.Context) ([]SyncRow, error) {
	const q = `
		WITH RECURSIVE cat_path AS (
			SELECT id, name::text AS path FROM categories WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, cp.path || ' > ' || c.name
			FROM categories c JOIN cat_path cp ON c.parent_id = cp.id
		)
		SELECT p.id, p.name, p.name_local, p.barcode,
		       COALESCE(cp.path, '') AS category,
		       u.abbreviation AS unit,
		       p.selling_price,
		       COALESCE(s.quantity, 0) AS stock_qty,
		       p.is_service
		FROM products p
		JOIN units u ON u.id = p.unit_id
		LEFT JOIN cat_path cp ON cp.id = p.category_id
		LEFT JOIN stock s ON s.product_id = p.id
		WHERE p.is_active = true
		ORDER BY p.name, p.id`
	var rows []SyncRow
	err := r.db.SelectContext(ctx, &rows, q)
	return rows, err
}

func (r *Repository) Count(ctx context.Context, q ListQuery) (int, error) {
	var n int
	toks, raw, fuzzy := q.searchArgs()
	err := r.db.GetContext(ctx, &n, subcatsCTE+`
		SELECT COUNT(*) FROM products p
		LEFT JOIN stock s ON s.product_id = p.id
		WHERE (p.is_active = true OR $7) AND (p.is_service = false OR $6)
		  AND (`+searchClause+` OR p.id = ANY($9::bigint[]))
		  AND ($4::bigint IS NULL OR p.category_id IN (SELECT id FROM subcats))
		  AND ($5 = false OR COALESCE(s.quantity,0) <= p.reorder_level OR p.id = ANY($10::bigint[]))
		  AND ($8::bigint IS NULL OR p.preferred_supplier_id = $8)`,
		toks, raw, fuzzy, q.CategoryID, q.LowStock, q.IncludeServices, q.IncludeInactive, q.PreferredSupplierID,
		pq.Array(searchMatchIDs(ctx, q)), pq.Array(q.AlsoIDs))
	return n, err
}

// Rename updates just the name column. Returns sql.ErrNoRows if the id is unknown
// so the service can report a 404 rather than a silent no-op.
func (r *Repository) Rename(ctx context.Context, id int64, name string) error {
	var got int64
	return r.db.GetContext(ctx, &got,
		`UPDATE products SET name = $2, updated_at = now() WHERE id = $1 RETURNING id`, id, name)
}

type writeRow struct {
	Name                          string
	NameLocal, Barcode            *string
	CategoryID, UnitID            int64
	Cost, Selling, Wholesale, Tax decimal.Decimal
	Reorder                       int
	TrackSerial                   bool
	WarrantyMonths                int
	IsService                     bool   // non-stocked service line (plugin seam)
	NeedsReview                   bool   // set by the till quick-add; false for normal creates
	CreatedBy                     *int64 // the user who quick-added it (nil otherwise)
	PreferredSupplier             *int64 // optional default supplier (nil = none)
}

func (r *Repository) Insert(ctx context.Context, w writeRow) (int64, error) {
	var id int64
	err := r.db.GetContext(ctx, &id, `
		INSERT INTO products
			(name, name_local, barcode, category_id, unit_id,
			 cost_price, selling_price, wholesale_price, tax_rate, reorder_level,
			 track_serial, warranty_months, needs_review, created_by, preferred_supplier_id,
			 is_service)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id`,
		w.Name, w.NameLocal, w.Barcode, w.CategoryID, w.UnitID,
		w.Cost, w.Selling, w.Wholesale, w.Tax, w.Reorder,
		w.TrackSerial, w.WarrantyMonths, w.NeedsReview, w.CreatedBy, w.PreferredSupplier,
		w.IsService)
	return id, err
}

func (r *Repository) Update(ctx context.Context, id int64, w writeRow) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE products SET
			name=$1, name_local=$2, barcode=$3, category_id=$4, unit_id=$5,
			cost_price=$6, selling_price=$7, wholesale_price=$8, tax_rate=$9, reorder_level=$10,
			track_serial=$11, warranty_months=$12, preferred_supplier_id=$13
		WHERE id=$14 AND is_active = true`,
		w.Name, w.NameLocal, w.Barcode, w.CategoryID, w.UnitID,
		w.Cost, w.Selling, w.Wholesale, w.Tax, w.Reorder,
		w.TrackSerial, w.WarrantyMonths, w.PreferredSupplier, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) SoftDelete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE products SET is_active = false WHERE id = $1`, id)
	return err
}

func (r *Repository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE products SET is_active = true WHERE id = $1`, id)
	return err
}

// SetCost updates only a product's cost price (used by the stock-take screen when
// the admin enters the cost of opening stock).
func (r *Repository) SetCost(ctx context.Context, id int64, cost decimal.Decimal) error {
	_, err := r.db.ExecContext(ctx, `UPDATE products SET cost_price = $1 WHERE id = $2`, cost, id)
	return err
}

// SetPrices updates a product's selling and wholesale prices together (the
// stock-take screen and count-sheet import set these alongside cost/quantity).
func (r *Repository) SetPrices(ctx context.Context, id int64, selling, wholesale decimal.Decimal) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET selling_price = $1, wholesale_price = $2 WHERE id = $3`, selling, wholesale, id)
	return err
}

// SetBarcodeIfEmpty assigns a barcode only when the product currently has none,
// so a generate action can never overwrite an existing code. Returns whether a
// row was actually updated.
func (r *Repository) SetBarcodeIfEmpty(ctx context.Context, id int64, code string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE products SET barcode = $1 WHERE id = $2 AND barcode IS NULL`, code, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CountByPreferredSupplier counts products whose default supplier is this one —
// shown in the reassign dialog so the owner knows how many lines will move.
func (r *Repository) CountByPreferredSupplier(ctx context.Context, supplierID int64) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM products WHERE preferred_supplier_id = $1`, supplierID)
	return n, err
}

// ReassignPreferredSupplier moves every product from one default supplier to
// another (a distributor swap) in one statement, returning how many moved.
func (r *Repository) ReassignPreferredSupplier(ctx context.Context, fromID, toID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE products SET preferred_supplier_id = $1 WHERE preferred_supplier_id = $2`, toID, fromID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// CountNeedsReview is the number of active products still flagged for review
// (quick-added at the till). Powers the admin-panel badge.
func (r *Repository) CountNeedsReview(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n,
		`SELECT COUNT(*) FROM products WHERE needs_review = true AND is_active = true`)
	return n, err
}

// ListNeedsReview returns the products awaiting review, newest first, with the
// name of the user who quick-added each one.
func (r *Repository) ListNeedsReview(ctx context.Context) ([]Product, error) {
	var rows []Product
	err := r.db.SelectContext(ctx, &rows, `
		SELECT p.*, c.name AS category_name, u.abbreviation AS unit_abbr,
		       u.allow_decimal AS unit_allow_decimal,
		       COALESCE(s.quantity, 0) AS stock_qty,
		       cb.name AS created_by_name
		FROM products p
		JOIN categories c ON c.id = p.category_id
		JOIN units u      ON u.id = p.unit_id
		LEFT JOIN stock s ON s.product_id = p.id
		LEFT JOIN users cb ON cb.id = p.created_by
		WHERE p.is_active = true AND p.needs_review = true
		ORDER BY p.created_at DESC`)
	return rows, err
}

// ClearReview marks a product as reviewed (admin has finished its setup).
func (r *Repository) ClearReview(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET needs_review = false WHERE id = $1`, id)
	return err
}

// BackfillZeroCost sets the cost on this product's sale lines that were recorded
// with the placeholder cost 0 (quick-add). This corrects COGS/profit on those
// past sales once the admin enters the real cost during review. Returns the
// number of sale lines corrected.
func (r *Repository) BackfillZeroCost(ctx context.Context, productID int64, cost decimal.Decimal) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE sale_items SET cost_price = $1 WHERE product_id = $2 AND cost_price = 0`,
		cost, productID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func nullStr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
