package reports

import (
	"context"
	"database/sql"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/sales"

	"github.com/jmoiron/sqlx"
)

// The one source every revenue report counts from (docs/rules/reports.md):
// what was sold in a period and what came back in it, each on its own day
// (RPT-6, user's Q1, 2026-11-20). A report sums these rows; none counts a
// return its own way.

// saleSumsSQL is each of the period's non-void sales' own figures, worked out
// once (RPT-38): pt = its pass-through face, own = its own lines' nets (NULL when
// it has none), cost = its own lines' cost. Joined on sale_id, never looked up
// per line — a lookup per line walked the products list for every sale.
const saleSumsSQL = `(SELECT x.sale_id,
	       COALESCE(SUM(x.subtotal) FILTER (WHERE xp.pass_through), 0) AS pt,
	       SUM(x.subtotal) FILTER (WHERE NOT xp.pass_through) AS own,
	       COALESCE(SUM(x.quantity * COALESCE(x.cost_price,0)) FILTER (WHERE NOT xp.pass_through), 0) AS cost
	FROM sale_items x JOIN sales xs ON xs.id = x.sale_id JOIN products xp ON xp.id = x.product_id
	WHERE xs.status <> 'void' AND xs.created_at >= $1 AND xs.created_at < $2
	GROUP BY x.sale_id)`

// ownFactorSQL is what each Rs of the shop's own line net brought in before
// tax: (total − tax − pass-through face) ÷ the shop's own line nets. The whole
// bill discount lands on the shop's own lines — a pass-through line (airtime,
// a payout) is never revenue and never takes a share (RPT-3, RPT-12, Q3), so
// the lines add up to the P&L. Correlated on `s` and `t` (saleSumsSQL).
const ownFactorSQL = `COALESCE((s.total - s.tax - t.pt) / NULLIF(t.own, 0), 0)`

// refundPreTaxSQL turns a refund (tax in) into its part before tax; taxBackSQL
// is its tax part (RPT-17). Correlated on `s`.
const (
	refundPreTaxSQL = `COALESCE((s.total - s.tax - ` + sales.PayoutsSQL + `) / NULLIF(s.total - ` + sales.PayoutsSQL + `, 0), 1)`
	taxBackSQL      = `COALESCE(s.tax / NULLIF(s.total - ` + sales.PayoutsSQL + `, 0), 0)`
)

// flowSQL is one row per sold line of the shop's own goods in [$1,$2) and one
// per returned line given back in [$1,$2): kind 's' / 'r', at (when), sale_id,
// product_id, user_id (who sold / who gave the refund), qty, revenue and cost
// (negative for a return), no_cost (a stocked line with no recorded cost, RPT-4).
const flowSQL = `
	SELECT 's' AS kind, s.created_at AS at, s.id AS sale_id, si.product_id, s.cashier_id AS user_id,
	       si.quantity AS qty, si.subtotal * ` + ownFactorSQL + ` AS revenue,
	       si.quantity * COALESCE(si.cost_price,0) AS cost,
	       (NOT p.is_service AND COALESCE(si.cost_price,0) = 0) AS no_cost
	FROM sale_items si JOIN sales s ON s.id = si.sale_id JOIN products p ON p.id = si.product_id
	JOIN ` + saleSumsSQL + ` t ON t.sale_id = s.id
	WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
	UNION ALL
	SELECT 'r', r.created_at, s.id, ri.product_id, r.created_by,
	       -ri.quantity, -ri.refund_amount * ` + refundPreTaxSQL + `,
	       -ri.quantity * COALESCE(si.cost_price,0),
	       (NOT p.is_service AND COALESCE(si.cost_price,0) = 0)
	FROM sale_return_items ri JOIN sale_returns r ON r.id = ri.return_id JOIN sales s ON s.id = r.sale_id
	JOIN sale_items si ON si.id = ri.sale_item_id JOIN products p ON p.id = ri.product_id
	WHERE NOT p.pass_through AND s.status <> 'void' AND r.created_at >= $1 AND r.created_at < $2`

// salesInSQL is the period's non-void sales (for counts: a sale of airtime
// alone has no own-goods line but is still a sale).
const salesInSQL = `FROM sales s WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`

// soldSQL is one row per sale in [$1,$2) at the P&L's sold figures (RPT-1):
// at, n = 1, user_id (its cashier), revenue = total − tax − pass-through face,
// cost = its own lines' cost; returnedSQL is the flow's returns in [$1,$2) in
// the same columns (n = 0, user_id = who gave the refund). Together: what the
// time and people reports sum (RPT-10, RPT-14).
const (
	soldSQL = `SELECT s.created_at AS at, 1 AS n, s.cashier_id AS user_id,
		s.total - s.tax - COALESCE(t.pt, 0) AS revenue, COALESCE(t.cost, 0) AS cost
		FROM sales s LEFT JOIN ` + saleSumsSQL + ` t ON t.sale_id = s.id
		WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`
	returnedSQL = `SELECT f.at, 0 AS n, f.user_id, f.revenue, f.cost FROM (` + flowSQL + `) f WHERE f.kind = 'r'`
)

// Snapshot runs fn over one REPEATABLE READ, read-only transaction, so every
// figure a page reads sees the same moment (RPT-36): a page showing several
// reports reads them all through q. A Service already inside a transaction (a
// test's) runs fn as it is.
func (s *Service) Snapshot(ctx context.Context, fn func(q *Service) error) error {
	db, ok := s.db.(*sqlx.DB)
	if !ok {
		return fn(s)
	}
	tx, err := db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return apperr.Internal("failed to start a report", err)
	}
	defer func() { _ = tx.Rollback() }()
	return fn(&Service{db: tx})
}
