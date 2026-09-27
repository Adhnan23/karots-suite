package products

import (
	"context"

	"github.com/shopspring/decimal"
)

// CountSheetRow is one product line on the printable stock count sheet.
type CountSheetRow struct {
	ID           int64           `db:"id"`
	Name         string          `db:"name"`
	Barcode      *string         `db:"barcode"`
	Unit         string          `db:"unit"`
	CategoryPath string          `db:"category_path"` // "Drinks > Soft drinks"
	StockQty     decimal.Decimal `db:"stock_qty"`
}

// CountSheet lists active stocked products for a physical count, optionally
// narrowed to a category subtree (any depth — a parent pulls in every
// descendant) and/or a preferred supplier, ordered by category path then name
// so the sheet groups naturally into consecutive runs per category.
func (r *Repository) CountSheet(ctx context.Context, categoryID, supplierID *int64) ([]CountSheetRow, error) {
	const q = `
		WITH RECURSIVE cat_path AS (
			SELECT id, name::text AS path FROM categories WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, cp.path || ' > ' || c.name
			FROM categories c JOIN cat_path cp ON c.parent_id = cp.id
		), subcats AS (
			SELECT id FROM categories WHERE $1::bigint IS NOT NULL AND id = $1
			UNION ALL
			SELECT c.id FROM categories c JOIN subcats sc ON c.parent_id = sc.id
		)
		SELECT p.id, p.name, p.barcode, u.abbreviation AS unit,
		       COALESCE(cp.path, '') AS category_path,
		       COALESCE(s.quantity, 0) AS stock_qty
		FROM products p
		JOIN units u ON u.id = p.unit_id
		LEFT JOIN cat_path cp ON cp.id = p.category_id
		LEFT JOIN stock s ON s.product_id = p.id
		WHERE p.is_active AND NOT p.is_service
		  AND ($1::bigint IS NULL OR p.category_id IN (SELECT id FROM subcats))
		  AND ($2::bigint IS NULL OR p.preferred_supplier_id = $2)
		ORDER BY category_path, p.name, p.id`
	var rows []CountSheetRow
	err := r.db.SelectContext(ctx, &rows, q, categoryID, supplierID)
	return rows, err
}
