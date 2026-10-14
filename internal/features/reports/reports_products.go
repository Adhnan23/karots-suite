// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// TopProducts ranks products by net revenue or net quantity sold over a period
// (net = after returns). orderBy is "qty" or "revenue" (default); limit is capped
// to a sane range. order is chosen from a whitelist, so concatenating it is safe.
func (s *Service) TopProducts(ctx context.Context, from, to time.Time, orderBy string, limit int) ([]ProductRevenue, error) {
	order := "revenue"
	switch orderBy {
	case "qty":
		order = "qty"
	case "profit":
		order = "profit"
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []ProductRevenue
	q := `
		SELECT p.id AS product_id, p.name AS product_name,
		       SUM(si.quantity - si.returned_qty) AS qty,
		       SUM( ` + KeptRevenueSQL + ` ) AS revenue,
		       SUM( ` + KeptRevenueSQL + `
		            - (si.quantity - si.returned_qty) * si.cost_price ) AS profit
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY p.id, p.name
		HAVING SUM(si.quantity - si.returned_qty) > 0
		ORDER BY ` + order + ` DESC
		LIMIT $3`
	if err := s.db.SelectContext(ctx, &rows, q, from, to, limit); err != nil {
		return nil, apperr.Internal("failed to compute top products", err)
	}
	return rows, nil
}

// KeptRevenueSQL is what a sale line actually brought in for the units kept,
// before tax: the line net (item discount off) per unit × kept qty × the sale's
// pre-tax paid factor ((total − tax) ÷ its line nets), so a bill discount is shared
// out across the lines. Without the factor every line-level report (categories,
// products, services) read the bill discount as revenue the shop never got. Tax
// stays out as before (the Tax report holds it). Correlated on `si` and `s`.
const KeptRevenueSQL = `((si.subtotal / NULLIF(si.quantity,0)) * (si.quantity - si.returned_qty)
	* COALESCE((s.total - s.tax) / NULLIF((SELECT SUM(x.subtotal) FROM sale_items x WHERE x.sale_id = s.id), 0), 1))`

// grossMargin is gross profit as a percent of net revenue (0 when revenue is 0).
func grossMargin(profit, revenue decimal.Decimal) decimal.Decimal {
	if !revenue.IsPositive() {
		return decimal.Zero
	}
	return profit.Div(revenue).Mul(decimal.NewFromInt(100)).Round(2)
}

// CategoryProfit is net revenue/COGS/profit for one category in the period.
// CatID/ParentID let callers roll leaf categories up into their parents.
type CategoryProfit struct {
	CatID    int64           `db:"cat_id"    json:"cat_id"`
	ParentID *int64          `db:"parent_id" json:"parent_id,omitempty"`
	Category string          `db:"category"  json:"category"`
	Revenue  decimal.Decimal `db:"revenue"   json:"revenue"`
	COGS     decimal.Decimal `db:"cogs"      json:"cogs"`
	Profit   decimal.Decimal `db:"profit"    json:"profit"`
}

// ProfitByCategory groups net sales by product category for the period. When
// `cats` is non-empty, only those category names are included. Each row carries
// its category id + parent so the web layer can build a roll-up tree.
func (s *Service) ProfitByCategory(ctx context.Context, from, to time.Time, cats ...string) ([]CategoryProfit, error) {
	var rows []CategoryProfit
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT cat.id AS cat_id, cat.parent_id, cat.name AS category,
		       SUM( `+KeptRevenueSQL+` ) AS revenue,
		       SUM( (si.quantity - si.returned_qty) * si.cost_price ) AS cogs,
		       SUM( `+KeptRevenueSQL+`
		            - (si.quantity - si.returned_qty) * si.cost_price ) AS profit
		FROM sale_items si
		JOIN sales s     ON s.id = si.sale_id
		JOIN products p  ON p.id = si.product_id
		JOIN categories cat ON cat.id = p.category_id
		WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		  AND ($3::text[] IS NULL OR cardinality($3::text[]) = 0 OR cat.name = ANY($3::text[]))
		GROUP BY cat.id, cat.parent_id, cat.name
		HAVING SUM(si.quantity - si.returned_qty) > 0
		ORDER BY profit DESC`, from, to, pq.Array(cats)); err != nil {
		return nil, apperr.Internal("failed to load category profit", err)
	}
	return rows, nil
}

// CategoryNames lists all category names (for the report's category filter).
func (s *Service) CategoryNames(ctx context.Context) ([]string, error) {
	var names []string
	if err := s.db.SelectContext(ctx, &names, `SELECT name FROM categories ORDER BY name`); err != nil {
		return nil, apperr.Internal("failed to load categories", err)
	}
	return names, nil
}

// CatRef is a category with its parent link, for building the hierarchical
// category filter (a parent selects its children).
type CatRef struct {
	ID       int64  `db:"id"`
	Name     string `db:"name"`
	ParentID *int64 `db:"parent_id"`
}

// CategoriesFlat lists every category with its parent, name-ordered so children
// come out sorted under each parent when the tree is built.
func (s *Service) CategoriesFlat(ctx context.Context) ([]CatRef, error) {
	var rows []CatRef
	if err := s.db.SelectContext(ctx, &rows, `SELECT id, name, parent_id FROM categories ORDER BY name`); err != nil {
		return nil, apperr.Internal("failed to load categories", err)
	}
	return rows, nil
}

// ProductPeriodRow is one product's net qty + revenue for a single period.
type ProductPeriodRow struct {
	Day     time.Time       `db:"day"     json:"day"`
	Qty     decimal.Decimal `db:"qty"     json:"qty"`
	Revenue decimal.Decimal `db:"revenue" json:"revenue"`
}

// ProductSalesByPeriod is one product's net units + revenue grouped by day/week/
// month (shop time) over [from,to). Powers the per-product sales graph (with a last-year
// overlay obtained by calling it again over the shifted range).
func (s *Service) ProductSalesByPeriod(ctx context.Context, productID int64, from, to time.Time, gran string) ([]ProductPeriodRow, error) {
	switch gran {
	case "day", "week", "month":
	default:
		gran = "day"
	}
	var rows []ProductPeriodRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT date_trunc($4, s.created_at AT TIME ZONE $5) AS day,
		       COALESCE(SUM(si.quantity - si.returned_qty),0) AS qty,
		       COALESCE(SUM( `+KeptRevenueSQL+` ),0) AS revenue
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		WHERE s.status <> 'void' AND si.product_id = $1
		  AND s.created_at >= $2 AND s.created_at < $3
		GROUP BY 1 ORDER BY 1`, productID, from, to, gran, datetime.Location.String()); err != nil {
		return nil, apperr.Internal("failed to load product sales", err)
	}
	return rows, nil
}

// ProductQtySold returns net units sold per product over [from,to) for the given
// product ids — the demand input for reorder forecasting.
func (s *Service) ProductQtySold(ctx context.Context, ids []int64, from, to time.Time) (map[int64]decimal.Decimal, error) {
	out := make(map[int64]decimal.Decimal, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductID int64           `db:"product_id"`
		Qty       decimal.Decimal `db:"qty"`
	}
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT si.product_id, COALESCE(SUM(si.quantity - si.returned_qty),0) AS qty
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		WHERE s.status <> 'void' AND si.product_id = ANY($1)
		  AND s.created_at >= $2 AND s.created_at < $3
		GROUP BY si.product_id`, pq.Array(ids), from, to); err != nil {
		return nil, apperr.Internal("failed to load product demand", err)
	}
	for _, r := range rows {
		out[r.ProductID] = r.Qty
	}
	return out, nil
}

// ProductTotals is every product sold in [from,to) together, counted as
// TopProducts counts each one — the whole a ranked page is a share of.
type ProductTotals struct {
	Products int             `db:"products"`
	Qty      decimal.Decimal `db:"qty"`
	Revenue  decimal.Decimal `db:"revenue"`
	Profit   decimal.Decimal `db:"profit"`
}

func (s *Service) ProductTotals(ctx context.Context, from, to time.Time) (*ProductTotals, error) {
	var t ProductTotals
	if err := s.db.GetContext(ctx, &t, `
		SELECT COUNT(*) AS products, COALESCE(SUM(qty),0) AS qty, COALESCE(SUM(revenue),0) AS revenue, COALESCE(SUM(profit),0) AS profit
		FROM (
			SELECT SUM(si.quantity - si.returned_qty) AS qty,
			       SUM( `+KeptRevenueSQL+` ) AS revenue,
			       SUM( `+KeptRevenueSQL+`
			            - (si.quantity - si.returned_qty) * si.cost_price ) AS profit
			FROM sale_items si
			JOIN sales s ON s.id = si.sale_id
			JOIN products p ON p.id = si.product_id
			WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
			GROUP BY p.id
			HAVING SUM(si.quantity - si.returned_qty) > 0
		) x`, from, to); err != nil {
		return nil, apperr.Internal("failed to total products", err)
	}
	return &t, nil
}
