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

// ExpenseCatRow is one expense category's total for the expenses report.
type ExpenseCatRow struct {
	Category string          `db:"category" json:"category"`
	Count    int             `db:"count"    json:"count"`
	Total    decimal.Decimal `db:"total"    json:"total"`
}

// ExpensesByCategory totals recorded operating expenses per category over a
// period (by expense_date), ranked by amount. The P&L shows only the single
// expenses total; this is where it breaks down.
func (s *Service) ExpensesByCategory(ctx context.Context, from, to time.Time) ([]ExpenseCatRow, error) {
	var rows []ExpenseCatRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT COALESCE(NULLIF(category,''),'(uncategorised)') AS category,
		       COUNT(*) AS count, COALESCE(SUM(amount),0) AS total
		FROM expenses
		WHERE expense_date >= $1 AND expense_date < $2
		GROUP BY COALESCE(NULLIF(category,''),'(uncategorised)')
		ORDER BY total DESC`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute expenses by category", err)
	}
	return rows, nil
}

// ReorderNeed is one product at/under its reorder level, for the Top-up &
// Reorder readiness report.
type ReorderNeed struct {
	Category  string          `db:"category"`
	Product   string          `db:"product"`
	OnHand    decimal.Decimal `db:"on_hand"`
	Reorder   int             `db:"reorder_level"`
	CostPrice decimal.Decimal `db:"cost_price"`
}

// Shortfall is how many units short of the reorder level (never negative).
func (n ReorderNeed) Shortfall() decimal.Decimal {
	s := decimal.NewFromInt(int64(n.Reorder)).Sub(n.OnHand)
	if s.IsNegative() {
		return decimal.Zero
	}
	return s
}

// RestockCost is the shortfall valued at cost — the cash to reorder to level.
func (n ReorderNeed) RestockCost() decimal.Decimal { return n.Shortfall().Mul(n.CostPrice) }

// ReorderNeeds lists active, stock-tracked products at or under their reorder
// level (reorder_level > 0), optionally narrowed to a set of category names.
// Newest-need first isn't meaningful here, so it orders by category then product.
func (s *Service) ReorderNeeds(ctx context.Context, cats ...string) ([]ReorderNeed, error) {
	var rows []ReorderNeed
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT c.name AS category, p.name AS product,
		       COALESCE(st.quantity,0) AS on_hand, p.reorder_level, p.cost_price
		FROM products p
		JOIN categories c ON c.id = p.category_id
		LEFT JOIN stock st ON st.product_id = p.id
		WHERE p.is_active AND NOT p.is_service AND NOT p.pass_through
		  AND p.reorder_level > 0 AND COALESCE(st.quantity,0) <= p.reorder_level
		  AND ($1::text[] IS NULL OR cardinality($1::text[]) = 0 OR c.name = ANY($1::text[]))
		ORDER BY c.name, p.name`, pq.Array(cats)); err != nil {
		return nil, apperr.Internal("failed to load reorder needs", err)
	}
	return rows, nil
}

// SupplierSpendRow is one supplier's purchasing tally for the spend report.
type SupplierSpendRow struct {
	Supplier string          `db:"supplier" json:"supplier"`
	Orders   int             `db:"orders"   json:"orders"`
	Spend    decimal.Decimal `db:"spend"    json:"spend"`
	Paid     decimal.Decimal `db:"paid"     json:"paid"`
	Due      decimal.Decimal `db:"due"      json:"due"`
}

// SupplierSpend ranks suppliers by what you bought from them over a period
// (received GRNs, drafts excluded) — the supplier twin of Top Products, and the
// "who do we buy most from" cut neither Purchases (per-GRN) nor Supplier Dues
// (current payables) gives.
func (s *Service) SupplierSpend(ctx context.Context, from, to time.Time) ([]SupplierSpendRow, error) {
	var rows []SupplierSpendRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT sup.name AS supplier, COUNT(*) AS orders,
		       COALESCE(SUM(pu.total),0) AS spend,
		       COALESCE(SUM(pu.paid_amount),0) AS paid,
		       COALESCE(SUM(pu.total - pu.paid_amount),0) AS due
		FROM purchases pu JOIN suppliers sup ON sup.id = pu.supplier_id
		WHERE pu.status <> 'draft' AND pu.created_at >= $1 AND pu.created_at < $2
		GROUP BY sup.name
		ORDER BY spend DESC`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute supplier spend", err)
	}
	return rows, nil
}

// WarrantySummary is the periodised warranty-cost vs supplier-recovery picture.
type WarrantySummary struct {
	ReplacementCount int             `json:"replacement_count"`
	ReplacementCost  decimal.Decimal `json:"replacement_cost"`
	DamageCount      int             `json:"damage_count"`
	DamageCost       decimal.Decimal `json:"damage_cost"`
	Recoveries       []RecoveryRow   `json:"recoveries"`
	RecoveredValue   decimal.Decimal `json:"recovered_value"`
}

// RecoveryRow is recovery totals for one outcome.
type RecoveryRow struct {
	Outcome   string          `db:"outcome"   json:"outcome"`
	Count     int             `db:"count"     json:"count"`
	LossValue decimal.Decimal `db:"loss_value" json:"loss_value"`
	Recovered decimal.Decimal `db:"recovered" json:"recovered"`
}

// WarrantyAndRecovery summarises stock losses and supplier recoveries by period.
func (s *Service) WarrantyAndRecovery(ctx context.Context, from, to time.Time) (*WarrantySummary, error) {
	out := &WarrantySummary{}
	var moves []struct {
		Type  string          `db:"type"`
		Count int             `db:"count"`
		Cost  decimal.Decimal `db:"cost"`
	}
	if err := s.db.SelectContext(ctx, &moves, `
		SELECT type, COUNT(*) AS count, COALESCE(SUM(cost),0) AS cost
		FROM stock_movements
		WHERE type IN ('warranty_replacement','damage') AND created_at >= $1 AND created_at < $2
		GROUP BY type`, from, to); err != nil {
		return nil, apperr.Internal("failed to load losses", err)
	}
	for _, m := range moves {
		switch m.Type {
		case "warranty_replacement":
			out.ReplacementCount, out.ReplacementCost = m.Count, m.Cost
		case "damage":
			out.DamageCount, out.DamageCost = m.Count, m.Cost
		}
	}
	if err := s.db.SelectContext(ctx, &out.Recoveries, `
		SELECT outcome, COUNT(*) AS count,
		       COALESCE(SUM(loss_value),0) AS loss_value,
		       COALESCE(SUM(recovered_value),0) AS recovered
		FROM loss_recoveries
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY outcome ORDER BY outcome`, from, to); err != nil {
		return nil, apperr.Internal("failed to load recoveries", err)
	}
	for _, r := range out.Recoveries {
		out.RecoveredValue = out.RecoveredValue.Add(r.Recovered)
	}
	return out, nil
}

// TaxDayRow is one day's taxable base (net sales excl. tax) and tax charged.
type TaxDayRow struct {
	Day   time.Time       `db:"day"   json:"day"`
	Count int             `db:"count" json:"count"`
	Base  decimal.Decimal `db:"base"  json:"base"`
	Tax   decimal.Decimal `db:"tax"   json:"tax"`
}

// TaxSummary is the per-day and total tax charged on sales for a period.
type TaxSummary struct {
	Rows      []TaxDayRow     `json:"rows"`
	Count     int             `json:"count"`
	TotalBase decimal.Decimal `json:"total_base"`
	TotalTax  decimal.Decimal `json:"total_tax"`
}

// TaxSummary aggregates tax charged on (non-void) sales per day. Base is the
// pre-tax amount (total − tax); this reflects tax charged at sale time and is
// not net of subsequent returns.
func (s *Service) TaxSummary(ctx context.Context, from, to time.Time) (*TaxSummary, error) {
	var rows []TaxDayRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT date_trunc('day', created_at AT TIME ZONE $3) AS day,
		       COUNT(*) AS count,
		       COALESCE(SUM(total - tax),0) AS base,
		       COALESCE(SUM(tax),0) AS tax
		FROM sales
		WHERE status <> 'void' AND created_at >= $1 AND created_at < $2
		GROUP BY 1 ORDER BY 1`, from, to, datetime.Location.String()); err != nil {
		return nil, apperr.Internal("failed to load tax summary", err)
	}
	out := &TaxSummary{Rows: rows}
	for _, r := range rows {
		out.TotalBase = out.TotalBase.Add(r.Base)
		out.TotalTax = out.TotalTax.Add(r.Tax)
		out.Count += r.Count
	}
	return out, nil
}

// ServiceProfitRow is one service's whole economic picture for a period:
// what it earned, what its recipe consumed, and what running it cost.
type ServiceProfitRow struct {
	ProductID int64           `db:"product_id"`
	Name      string          `db:"name"`
	Units     decimal.Decimal `db:"units"`
	Revenue   decimal.Decimal `db:"revenue"`
	COGS      decimal.Decimal `db:"cogs"`
	Expenses  decimal.Decimal `db:"expenses"`
}

// GrossProfit is revenue less what the recipe consumed.
func (r ServiceProfitRow) GrossProfit() decimal.Decimal { return r.Revenue.Sub(r.COGS) }

// NetProfit also subtracts expenses attributed to this service — the toner
// change, the machine repair. Those are not per-unit costs, so they never enter
// COGS; they are subtracted once, here, where the question is "did this counter
// pay for itself".
func (r ServiceProfitRow) NetProfit() decimal.Decimal { return r.GrossProfit().Sub(r.Expenses) }

// MarginPct is net profit as a percentage of revenue; zero when nothing sold.
func (r ServiceProfitRow) MarginPct() decimal.Decimal {
	if r.Revenue.IsZero() {
		return decimal.Zero
	}
	return r.NetProfit().Div(r.Revenue).Mul(decimal.NewFromInt(100)).Round(1)
}

// ServiceProfit reports each service product's income, ingredient cost and
// attributed running costs for a period.
//
// Services are absent from every stock-facing report (they hold no inventory),
// and the shop-wide P&L blends them into one number, so there was no way to ask
// "what did the coffee machine actually make me". A service with no sales but
// real expenses still appears: a counter that cost money and earned nothing is
// exactly what this report has to be able to show.
func (s *Service) ServiceProfit(ctx context.Context, from, to time.Time) ([]ServiceProfitRow, error) {
	var rows []ServiceProfitRow
	if err := s.db.SelectContext(ctx, &rows, `
		WITH sold AS (
			SELECT si.product_id,
			       SUM(si.quantity - si.returned_qty) AS units,
			       SUM( (si.subtotal / NULLIF(si.quantity,0)) * (si.quantity - si.returned_qty) ) AS revenue,
			       SUM( (si.quantity - si.returned_qty) * si.cost_price ) AS cogs
			FROM sale_items si
			JOIN sales sa ON sa.id = si.sale_id
			WHERE sa.status <> 'void' AND sa.created_at >= $1 AND sa.created_at < $2
			GROUP BY si.product_id
		),
		tagged AS (
			SELECT service_product_id AS product_id, SUM(amount) AS expenses
			FROM expenses
			WHERE service_product_id IS NOT NULL
			  AND expense_date >= $1 AND expense_date < $2
			GROUP BY service_product_id
		)
		SELECT p.id AS product_id, p.name,
		       COALESCE(so.units,0)    AS units,
		       COALESCE(so.revenue,0)  AS revenue,
		       COALESCE(so.cogs,0)     AS cogs,
		       COALESCE(tg.expenses,0) AS expenses
		FROM products p
		LEFT JOIN sold   so ON so.product_id = p.id
		LEFT JOIN tagged tg ON tg.product_id = p.id
		WHERE p.is_service
		  AND (so.product_id IS NOT NULL OR tg.product_id IS NOT NULL)
		ORDER BY COALESCE(so.revenue,0) - COALESCE(so.cogs,0) - COALESCE(tg.expenses,0) DESC`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to load service profit", err)
	}
	return rows, nil
}
