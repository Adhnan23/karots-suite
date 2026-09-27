// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"

	"github.com/shopspring/decimal"
)

// CashierSalesRow is one cashier's sales tally for the sales-by-cashier report.
type CashierSalesRow struct {
	Cashier  string          `db:"cashier"  json:"cashier"`
	Count    int             `db:"count"    json:"count"`
	Gross    decimal.Decimal `db:"gross"    json:"gross"`
	Discount decimal.Decimal `db:"discount" json:"discount"`
	Net      decimal.Decimal `db:"net"      json:"net"`
}

// SalesByCashier totals sales per cashier over a period (void sales excluded),
// ranked by net takings — the staff-performance cut Cash Register (drawer
// over/short only) doesn't give.
func (s *Service) SalesByCashier(ctx context.Context, from, to time.Time) ([]CashierSalesRow, error) {
	var rows []CashierSalesRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT u.name AS cashier, COUNT(*) AS count,
		       COALESCE(SUM(s.subtotal),0) - COALESCE(SUM(pt.amt),0) AS gross,
		       COALESCE(SUM(s.discount),0) AS discount,
		       COALESCE(SUM(s.total),0)    - COALESCE(SUM(pt.amt),0) AS net
		FROM sales s JOIN users u ON u.id = s.cashier_id
		LEFT JOIN (
			SELECT si.sale_id, SUM(si.subtotal) AS amt
			FROM sale_items si JOIN products p ON p.id = si.product_id
			WHERE p.pass_through GROUP BY si.sale_id
		) pt ON pt.sale_id = s.id
		WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY u.name ORDER BY net DESC`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute sales by cashier", err)
	}
	return rows, nil
}

// ReturnRow is one returned line for the returns report.
type ReturnRow struct {
	SaleDate    time.Time       `db:"sale_date"    json:"sale_date"`
	ReceiptNo   string          `db:"receipt_no"   json:"receipt_no"`
	ProductName string          `db:"product_name" json:"product_name"`
	Qty         decimal.Decimal `db:"qty"          json:"qty"`
	RefundValue decimal.Decimal `db:"refund_value" json:"refund_value"`
	Customer    *string         `db:"customer"     json:"customer,omitempty"`
}

// Returns lists returned lines (full or partial) for sales in the period.
func (s *Service) Returns(ctx context.Context, from, to time.Time) ([]ReturnRow, error) {
	var rows []ReturnRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT s.created_at AS sale_date, s.receipt_no, p.name AS product_name,
		       si.returned_qty AS qty,
		       ((si.subtotal / NULLIF(si.quantity,0)) * si.returned_qty) AS refund_value,
		       c.name AS customer
		FROM sale_items si
		JOIN sales s     ON s.id = si.sale_id
		JOIN products p  ON p.id = si.product_id
		LEFT JOIN customers c ON c.id = s.customer_id
		WHERE si.returned_qty > 0 AND s.status <> 'void'
		  AND s.created_at >= $1 AND s.created_at < $2
		ORDER BY s.created_at DESC, s.receipt_no`, from, to); err != nil {
		return nil, apperr.Internal("failed to load returns", err)
	}
	return rows, nil
}

// DayRow is one day's net sales for the trend report.
type DayRow struct {
	Day     time.Time       `db:"day"     json:"day"`
	Count   int             `db:"count"   json:"count"`
	Revenue decimal.Decimal `db:"revenue" json:"revenue"`
	Profit  decimal.Decimal `db:"profit"  json:"profit"`
}

// DailySales is per-day net revenue and profit for the period.
func (s *Service) DailySales(ctx context.Context, from, to time.Time) ([]DayRow, error) {
	return s.SalesByPeriod(ctx, from, to, "day")
}

// SalesByPeriod is net revenue/profit grouped by day, week or month. `gran` must
// be one of day/week/month; anything else falls back to day. Each row's Day is
// the period start (truncated date).
func (s *Service) SalesByPeriod(ctx context.Context, from, to time.Time, gran string) ([]DayRow, error) {
	switch gran {
	case "day", "week", "month":
	default:
		gran = "day"
	}
	var rows []DayRow
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT d.day, d.count, d.revenue, d.revenue - d.cogs AS profit
		FROM (
			SELECT date_trunc($3, s.created_at) AS day,
			       COUNT(DISTINCT s.id) AS count,
			       COALESCE(SUM( (si.subtotal / NULLIF(si.quantity,0)) * (si.quantity - si.returned_qty) ),0) AS revenue,
			       COALESCE(SUM( (si.quantity - si.returned_qty) * si.cost_price ),0) AS cogs
			FROM sales s
			JOIN sale_items si ON si.sale_id = s.id
			WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
			GROUP BY 1
		) d
		ORDER BY d.day`, from, to, gran); err != nil {
		return nil, apperr.Internal("failed to load sales by period", err)
	}
	return rows, nil
}

// PeakBucket is sales activity in one day-of-week × hour-of-day cell, in the
// shop's local timezone. DOW is Postgres' 0=Sunday … 6=Saturday.
type PeakBucket struct {
	DOW     int             `db:"dow"`
	Hour    int             `db:"hour"`
	Count   int             `db:"count"`
	Revenue decimal.Decimal `db:"revenue"`
}

// PeakHours buckets non-void sales in [from,to) by local day-of-week and
// hour-of-day — the staffing view (when are we busy). Timezone comes from the
// same datetime package the rest of the app displays with.
func (s *Service) PeakHours(ctx context.Context, from, to time.Time) ([]PeakBucket, error) {
	tz := datetime.Location.String()
	var rows []PeakBucket
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT EXTRACT(DOW  FROM (created_at AT TIME ZONE $3))::int AS dow,
		       EXTRACT(HOUR FROM (created_at AT TIME ZONE $3))::int AS hour,
		       COUNT(*) AS count,
		       COALESCE(SUM(total),0) AS revenue
		FROM sales
		WHERE status <> 'void' AND created_at >= $1 AND created_at < $2
		GROUP BY 1, 2`, from, to, tz); err != nil {
		return nil, apperr.Internal("failed to load peak hours", err)
	}
	return rows, nil
}
