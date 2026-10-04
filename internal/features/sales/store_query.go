// Package sales records POS transactions. A sale is written atomically: stock
// is decremented under a guard (no oversell), movements are audited, and any
// credit portion updates the customer balance — all in one transaction.
package sales

import (
	"context"
	"time"

	"karots-suite/internal/datetime"

	"github.com/shopspring/decimal"
)

// ListFilter narrows the sales list for the admin panel.
type ListFilter struct {
	From      *time.Time
	To        *time.Time
	CashierID *int64
	Status    string
	Receipt   string // receipt-number substring match (blank = any)
	Query     string // matches receipt no / customer name / customer phone (blank = any)
	Method    string // payment method on the sale (blank = any)
	// FromHour/ToHour restrict sales to an hour-of-day window in the shop's local
	// timezone (inclusive both ends, 0–23). nil = no time-of-day restriction.
	FromHour *int
	ToHour   *int
	Limit    int
	Offset   int
}

// tzName is the shop-local zone passed into SQL for hour bucketing; it is the
// same zone the datetime package uses to display timestamps.
func tzName() string { return datetime.Location.String() }

// MaxListLimit is the largest page List will serve.
const MaxListLimit = 500

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Sale, error) {
	// Clamp DOWN TO the maximum, never below it: the old "> 500 → 100" rule
	// meant a caller asking for 10000 got 100, and any total summed from the
	// returned rows silently omitted the rest.
	switch {
	case f.Limit <= 0:
		f.Limit = 100
	case f.Limit > MaxListLimit:
		f.Limit = MaxListLimit
	}
	var status *string
	if f.Status != "" {
		status = &f.Status
	}
	var receipt *string
	if f.Receipt != "" {
		receipt = &f.Receipt
	}
	var method *string
	if f.Method != "" {
		method = &f.Method
	}
	var query *string
	if f.Query != "" {
		query = &f.Query
	}
	var rows []Sale
	err := r.q.SelectContext(ctx, &rows, `
		SELECT s.*, u.name AS cashier_name, c.name AS customer_name,
		       agg.cogs AS cogs,
		       (s.total - agg.pt_face - agg.ret_val - agg.cogs) AS profit,
		       CASE WHEN (s.total - agg.pt_face - agg.ret_val) > 0
		            THEN (s.total - agg.pt_face - agg.ret_val - agg.cogs)
		                 / (s.total - agg.pt_face - agg.ret_val) * 100
		            ELSE 0 END AS margin
		FROM sales s
		JOIN users u ON u.id = s.cashier_id
		LEFT JOIN customers c ON c.id = s.customer_id
		JOIN LATERAL (`+saleProfitAgg+`) agg ON true
		WHERE ($1::timestamptz IS NULL OR s.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR s.created_at <  $2)
		  AND ($3::bigint IS NULL OR s.cashier_id = $3)
		  AND ($4::text IS NULL OR s.status = $4::sale_status)
		  AND ($5::text IS NULL OR s.receipt_no ILIKE '%' || $5 || '%')
		  AND ($8::text IS NULL OR EXISTS (
		      SELECT 1 FROM payments p WHERE p.sale_id = s.id AND p.method = $8::payment_method))
		  AND `+searchMatch("$9")+hourClause("s", "$10", "$11", "$12")+`
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $6 OFFSET $7`,
		f.From, f.To, f.CashierID, status, receipt, f.Limit, f.Offset, method, query,
		tzName(), f.FromHour, f.ToHour)
	return rows, err
}

// searchMatch is the sales search (receipt no / customer name / phone / what
// was sold), shared by List and Summarize so the totals match the rows. p is the
// query's placeholder, e.g. "$9". A sale matches by a line when every word of
// the query appears in that ONE line's product name + label, in any order, with
// separators squashed ("mineral 1l" → "Mineral Water 1L", "flipflop" → "Flip
// Flop") — the till's rules minus typo rescue, which on sales history surfaces
// unrelated receipts. A scanned barcode matches exactly. EXISTS, so a sale with
// several matching lines is listed (and counted) once. ' -_/.' mirrors
// products.squashChars; keep them in sync.
func searchMatch(p string) string {
	return `(` + p + `::text IS NULL OR
	       s.receipt_no ILIKE '%' || ` + p + ` || '%' OR
	       c.name       ILIKE '%' || ` + p + ` || '%' OR
	       c.phone      ILIKE '%' || ` + p + ` || '%' OR
	       EXISTS (SELECT 1 FROM sale_items si JOIN products pr ON pr.id = si.product_id
	               WHERE si.sale_id = s.id
	                 AND (pr.barcode = ` + p + ` OR NOT EXISTS (
	                      SELECT 1 FROM regexp_split_to_table(lower(btrim(` + p + `::text)), '\s+') AS tok
	                      WHERE NOT (
	                          lower(pr.name || ' ' || COALESCE(si.description, '')) LIKE '%' || tok || '%'
	                          OR translate(lower(pr.name || ' ' || COALESCE(si.description, '')), ' -_/.', '')
	                             LIKE '%' || translate(tok, ' -_/.', '') || '%')))))`
}

// saleProfitAgg is the per-sale cost/pass-through/returns aggregate, mirroring
// the Finance P&L definitions. Correlated on the outer `s`.
const saleProfitAgg = `
	SELECT
	  COALESCE(SUM(si.subtotal) FILTER (WHERE p.pass_through), 0)                                          AS pt_face,
	  COALESCE(SUM((si.subtotal / NULLIF(si.quantity,0)) * si.returned_qty) FILTER (WHERE NOT p.pass_through), 0) * ` + PaidFactorSQL + ` AS ret_val,
	  COALESCE(SUM((si.quantity - si.returned_qty) * si.cost_price) FILTER (WHERE NOT p.pass_through), 0)  AS cogs
	FROM sale_items si JOIN products p ON p.id = si.product_id
	WHERE si.sale_id = s.id`

// hourClause returns the shop-local hour-of-day window predicate (inclusive both
// ends), or empty when both hour params are nil. tzP/fromP/toP are the SQL
// placeholders (e.g. "$10"). alias is the sales-table alias.
func hourClause(alias, tzP, fromP, toP string) string {
	h := "EXTRACT(HOUR FROM (" + alias + ".created_at AT TIME ZONE " + tzP + "))::int"
	return `
		  AND (` + fromP + `::int IS NULL OR ` + h + ` >= ` + fromP + `)
		  AND (` + toP + `::int IS NULL OR ` + h + ` <= ` + toP + `)`
}

// ListSummary is the whole-filter aggregate behind a page of List results.
// Reports must take their totals from here, never by summing a rendered page.
type ListSummary struct {
	Count    int             `db:"count"`
	Gross    decimal.Decimal `db:"gross"`
	Discount decimal.Decimal `db:"discount"`
	Net      decimal.Decimal `db:"net"`
	COGS     decimal.Decimal `db:"cogs"`
	Profit   decimal.Decimal `db:"profit"`
	// NetRev is revenue net of pass-through face value and returns — the base for
	// margin, matching the per-row definition. Net (SUM of total) stays the
	// headline "sales total" and is not used for margin.
	NetRev decimal.Decimal `db:"net_rev"`
}

// Margin is gross margin percent over net revenue. Zero when there is no revenue.
func (s ListSummary) Margin() decimal.Decimal {
	if !s.NetRev.IsPositive() {
		return decimal.Zero
	}
	return s.Profit.Div(s.NetRev).Mul(decimal.NewFromInt(100))
}

// AvgBasket is net revenue per sale (zero when there are no sales).
func (s ListSummary) AvgBasket() decimal.Decimal {
	if s.Count == 0 {
		return decimal.Zero
	}
	return s.Net.Div(decimal.NewFromInt(int64(s.Count)))
}

// Summarize aggregates every sale matching f, ignoring Limit/Offset entirely.
// The WHERE clause is kept identical to List above — change one, change both.
func (r *Repository) Summarize(ctx context.Context, f ListFilter) (*ListSummary, error) {
	var status *string
	if f.Status != "" {
		status = &f.Status
	}
	var receipt *string
	if f.Receipt != "" {
		receipt = &f.Receipt
	}
	var method *string
	if f.Method != "" {
		method = &f.Method
	}
	var query *string
	if f.Query != "" {
		query = &f.Query
	}
	var out ListSummary
	err := r.q.GetContext(ctx, &out, `
		SELECT count(*)                                         AS count,
		       COALESCE(SUM(s.subtotal), 0)                     AS gross,
		       COALESCE(SUM(s.discount), 0)                     AS discount,
		       COALESCE(SUM(s.total), 0)                        AS net,
		       COALESCE(SUM(agg.cogs), 0)                       AS cogs,
		       COALESCE(SUM(s.total - agg.pt_face - agg.ret_val - agg.cogs), 0) AS profit,
		       COALESCE(SUM(s.total - agg.pt_face - agg.ret_val), 0)           AS net_rev
		FROM sales s
		JOIN users u ON u.id = s.cashier_id
		LEFT JOIN customers c ON c.id = s.customer_id
		JOIN LATERAL (`+saleProfitAgg+`) agg ON true
		WHERE ($1::timestamptz IS NULL OR s.created_at >= $1)
		  AND ($2::timestamptz IS NULL OR s.created_at <  $2)
		  AND ($3::bigint IS NULL OR s.cashier_id = $3)
		  AND ($4::text IS NULL OR s.status = $4::sale_status)
		  AND ($5::text IS NULL OR s.receipt_no ILIKE '%' || $5 || '%')
		  AND ($6::text IS NULL OR EXISTS (
		      SELECT 1 FROM payments p WHERE p.sale_id = s.id AND p.method = $6::payment_method))
		  AND `+searchMatch("$7")+hourClause("s", "$8", "$9", "$10"),
		f.From, f.To, f.CashierID, status, receipt, method, query,
		tzName(), f.FromHour, f.ToHour)
	return &out, err
}
