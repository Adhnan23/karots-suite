package products

import (
	"context"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// ValuedProduct is one product's stock valued the way the valuation totals
// value it — by the lots it is made of, the product price only for stock no lot
// accounts for — so the rows add up to the headline. (karots-pos listed
// quantity × the current cost price, which disagreed with its own total
// whenever a lot was bought at another price.)
type ValuedProduct struct {
	ID          int64           `db:"id"`
	Name        string          `db:"name"`
	Barcode     *string         `db:"barcode"`
	Category    string          `db:"category"`
	Unit        string          `db:"unit"`
	Qty         decimal.Decimal `db:"qty"`
	CostValue   decimal.Decimal `db:"cost_value"`
	RetailValue decimal.Decimal `db:"retail_value"`
}

// ValuedProducts lists the products of a branch (the whole shop when
// q.CategoryID is nil), most valuable first. limit <= 0 = every one.
func (s *Service) ValuedProducts(ctx context.Context, q ValuationQuery, limit, offset int) ([]ValuedProduct, error) {
	var rows []ValuedProduct
	if err := s.repo.db.SelectContext(ctx, &rows, branchCTE+`
		SELECT p.id, p.name, p.barcode, COALESCE(c.name, '') AS category, COALESCE(u.abbreviation, '') AS unit,
		       COALESCE(st.quantity, 0) AS qty,
		       COALESCE(lv.lot_cost,0) + (COALESCE(st.quantity,0) - COALESCE(lv.lot_qty,0)) * COALESCE(p.cost_price,0) AS cost_value,
		       COALESCE(lv.lot_retail,0) + (COALESCE(st.quantity,0) - COALESCE(lv.lot_qty,0)) * COALESCE(p.selling_price,0) AS retail_value
		FROM sub s
		JOIN products p    ON p.category_id = s.id AND p.is_active AND NOT p.is_service
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN units u  ON u.id = p.unit_id
		LEFT JOIN stock st ON st.product_id = p.id`+lotValueJoin+`
		WHERE ($2 = true OR COALESCE(st.quantity,0) <> 0)
		ORDER BY cost_value DESC, p.name, p.id
		LIMIT NULLIF($3, 0) OFFSET $4`, q.CategoryID, q.IncludeZero, limit, offset); err != nil {
		return nil, apperr.Internal("failed to value products", err)
	}
	return rows, nil
}
