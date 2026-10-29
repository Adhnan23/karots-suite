// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	"karots-suite/internal/features/sales"

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
		       COALESCE(SUM(s.total - s.tax),0) - COALESCE(SUM(pt.amt),0) AS net
		FROM sales s JOIN users u ON u.id = s.cashier_id
		LEFT JOIN (
			SELECT si.sale_id, SUM(si.subtotal) AS amt
			FROM sale_items si JOIN products p ON p.id = si.product_id
			WHERE p.pass_through GROUP BY si.sale_id
		) pt ON pt.sale_id = s.id
		WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY u.id, u.name ORDER BY net DESC`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute sales by cashier", err)
	}
	return rows, nil
}

// ReturnLine is one returned line, dated when the money went back (the
// return), with its sale's date beside it. Refund is what was actually given
// for it (sale_return_items, the bill discount already shared out).
type ReturnLine struct {
	When      time.Time       `db:"created_at"`
	SaleID    int64           `db:"sale_id"`
	ReceiptNo string          `db:"receipt_no"`
	SaleDate  time.Time       `db:"sale_date"`
	Product   string          `db:"product"`
	Qty       decimal.Decimal `db:"qty"`
	Refund    decimal.Decimal `db:"refund"`
	Customer  *string         `db:"customer"`
	Reason    *string         `db:"reason"`
}

// ReturnTotals is every return made in [from,to).
type ReturnTotals struct {
	Returns  int             `db:"returns"`
	Receipts int             `db:"receipts"`
	Qty      decimal.Decimal `db:"qty"`
	Refund   decimal.Decimal `db:"refund"`
}

// ReturnProduct is one product's returns in range.
type ReturnProduct struct {
	Name   string          `db:"name"`
	Qty    decimal.Decimal `db:"qty"`
	Refund decimal.Decimal `db:"refund"`
}

const returnsFrom = `
		FROM sale_return_items ri
		JOIN sale_returns r   ON r.id = ri.return_id
		JOIN sales s          ON s.id = r.sale_id
		JOIN products p       ON p.id = ri.product_id
		LEFT JOIN customers c ON c.id = s.customer_id
		WHERE r.created_at >= $1 AND r.created_at < $2`

// ReturnsIn totals the returns made in [from,to) — by return date, so a refund
// given today on last month's sale is today's.
func (s *Service) ReturnsIn(ctx context.Context, from, to time.Time) (*ReturnTotals, error) {
	var t ReturnTotals
	if err := s.db.GetContext(ctx, &t, `
		SELECT COUNT(DISTINCT r.id) AS returns, COUNT(DISTINCT r.sale_id) AS receipts,
		       COALESCE(SUM(ri.quantity),0) AS qty, COALESCE(SUM(ri.refund_amount),0) AS refund`+returnsFrom, from, to); err != nil {
		return nil, apperr.Internal("failed to total returns", err)
	}
	return &t, nil
}

// MostReturned is the products with the most refunded in [from,to).
func (s *Service) MostReturned(ctx context.Context, from, to time.Time, limit int) ([]ReturnProduct, error) {
	var rows []ReturnProduct
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT p.name, SUM(ri.quantity) AS qty, SUM(ri.refund_amount) AS refund`+returnsFrom+`
		GROUP BY p.id, p.name ORDER BY refund DESC, qty DESC LIMIT $3`, from, to, limit); err != nil {
		return nil, apperr.Internal("failed to rank returned products", err)
	}
	return rows, nil
}

// ReturnLines pages the returned lines in [from,to), newest first. limit 0 =
// every line (the export).
func (s *Service) ReturnLines(ctx context.Context, from, to time.Time, limit, offset int) ([]ReturnLine, error) {
	var rows []ReturnLine
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT r.created_at, s.id AS sale_id, s.receipt_no, s.created_at AS sale_date, p.name AS product,
		       ri.quantity AS qty, ri.refund_amount AS refund, c.name AS customer, r.reason`+returnsFrom+`
		ORDER BY r.created_at DESC, ri.id DESC
		LIMIT NULLIF($3, 0) OFFSET $4`, from, to, limit, offset); err != nil {
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

// SalesByPeriod is net revenue/profit grouped by day, week or month. `gran` must
// be one of day/week/month; anything else falls back to day. Each row's Day is
// the period start in shop time. Revenue is counted exactly as Compute counts it
// (sale total before tax, less pass-through face and returns at what was paid, via the same
// per-sale aggregate the sales list uses), so the periods add up to the P&L.
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
			SELECT date_trunc($3, s.created_at AT TIME ZONE $4) AS day,
			       COUNT(*) AS count,
			       COALESCE(SUM(s.total - s.tax - agg.pt_face - agg.ret_val * `+preTaxShareSQL+`),0) AS revenue,
			       COALESCE(SUM(agg.cogs),0) AS cogs
			FROM sales s
			JOIN LATERAL (`+sales.ProfitAggSQL+`) agg ON true
			WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
			GROUP BY 1
		) d
		ORDER BY d.day`, from, to, gran, datetime.Location.String()); err != nil {
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
// hour-of-day, revenue counted as the P&L counts it — the staffing view (when are we busy). Timezone comes from the
// same datetime package the rest of the app displays with.
func (s *Service) PeakHours(ctx context.Context, from, to time.Time) ([]PeakBucket, error) {
	tz := datetime.Location.String()
	var rows []PeakBucket
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT EXTRACT(DOW  FROM (s.created_at AT TIME ZONE $3))::int AS dow,
		       EXTRACT(HOUR FROM (s.created_at AT TIME ZONE $3))::int AS hour,
		       COUNT(*) AS count,
		       COALESCE(SUM(s.total - s.tax - agg.pt_face - agg.ret_val * `+preTaxShareSQL+`),0) AS revenue
		FROM sales s
		JOIN LATERAL (`+sales.ProfitAggSQL+`) agg ON true
		WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY 1, 2 ORDER BY 1, 2`, from, to, tz); err != nil {
		return nil, apperr.Internal("failed to load peak hours", err)
	}
	return rows, nil
}
