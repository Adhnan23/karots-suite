// Package datacheck lists stored data that older karots-pos versions wrote
// wrongly, before the fixes found while porting to the suite. It only reads:
// a restored shop sees what is off and the owner decides what to repair.
//
// Each check names the karots-pos fix that stopped new damage. Bugs that only
// misread correct data (reports, statements) need no check — the suite reads
// them right. Add a check here when a fix lands for a bug that WROTE bad rows.
package datacheck

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// Row is one example of a finding.
type Row struct {
	Ref    string `db:"ref"`
	Detail string `db:"detail"`
}

// Finding is one check that found something.
type Finding struct {
	Key     string
	Title   string
	Explain string
	Count   int
	Sample  []Row // at most sampleSize
}

const sampleSize = 20

type check struct {
	key, title, explain string
	// query returns (ref, detail) for every bad row, newest first.
	query string
}

var checks = []check{
	{
		key:   "over_credited_returns",
		title: "Returns took more off a customer's debt than the sale left owing",
		explain: "Before karots-pos d55ac5f, each return on a part-paid credit sale could clear up to the whole unpaid amount, " +
			"so a second return lowered the customer's balance again and gave no cash back. The customer's balance is too low by the excess.",
		query: `
SELECT s.receipt_no AS ref,
       c.name || ': owed ' || to_char(s.total - s.paid_amount, 'FM999,999,990.00') ||
       ', returns took ' || to_char(SUM(sr.credit_reduction), 'FM999,999,990.00') AS detail
FROM sales s JOIN sale_returns sr ON sr.sale_id = s.id
LEFT JOIN customers c ON c.id = s.customer_id
GROUP BY s.id, c.name
HAVING SUM(sr.credit_reduction) > GREATEST(s.total - s.paid_amount, 0) + 0.01
ORDER BY s.created_at DESC`,
	},
	{
		key:   "refund_over_paid",
		title: "Refunds came to more than the customer paid",
		explain: "Before karots-pos b256484, a return refunded the sticker price and ignored the bill discount, " +
			"so returning a discounted sale gave back more than was paid. The extra left the drawer or came off a balance.",
		query: `
SELECT s.receipt_no AS ref,
       'paid ' || to_char(s.total, 'FM999,999,990.00') || ', refunded ' ||
       to_char(SUM(sr.refund_amount + sr.credit_reduction), 'FM999,999,990.00') AS detail
FROM sales s JOIN sale_returns sr ON sr.sale_id = s.id
GROUP BY s.id
HAVING SUM(sr.refund_amount + sr.credit_reduction) > s.total + 0.01
ORDER BY s.created_at DESC`,
	},
	{
		key:   "returned_without_record",
		title: "Sales marked returned with no return recorded",
		explain: "Before karots-pos 3f77ef3, a whole-sale return put the stock back and dropped the customer's credit " +
			"but recorded no return or refund, so it is missing from the Returns report, the refund slip and the Z report.",
		query: `
SELECT s.receipt_no AS ref,
       'total ' || to_char(s.total, 'FM999,999,990.00') || ', ' || to_char(s.created_at, 'YYYY-MM-DD') AS detail
FROM sales s
WHERE s.status = 'returned' AND NOT EXISTS (SELECT 1 FROM sale_returns sr WHERE sr.sale_id = s.id)
ORDER BY s.created_at DESC`,
	},
	{
		key:   "deleted_expense_money",
		title: "Money paid out for expenses that were later deleted",
		explain: "Before karots-pos d66ac18, deleting an expense removed the row but left the money out of the locker, " +
			"so the locker is short by these amounts with no expense to explain it.",
		query: `
SELECT 'Locker ' || l.name AS ref,
       to_char(-SUM(ll.balance_delta), 'FM999,999,990.00') || ' on ' || to_char(MIN(ll.created_at), 'YYYY-MM-DD') ||
       COALESCE(' — ' || NULLIF(MIN(ll.note), ''), '') AS detail
FROM locker_ledger ll JOIN lockers l ON l.id = ll.locker_id
WHERE ll.ref_kind = 'expense' AND ll.ref_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM expenses e WHERE e.id = ll.ref_id)
GROUP BY l.id, ll.ref_id
-- A voided expense (the fix) is deleted with its money put back under the
-- same ref, so its legs net to zero; only money never returned is reported.
HAVING SUM(ll.balance_delta) <> 0
ORDER BY MIN(ll.created_at) DESC`,
	},
	{
		key:   "change_beyond_cash",
		title: "Change given back on card or online money",
		explain: "Before karots-pos c5553e7, a card or online payment above the bill was accepted and the difference paid out " +
			"of the drawer as change. Card / online takings read high and the drawer paid out cash it never took.",
		query: `
SELECT s.receipt_no AS ref,
       'change ' || to_char(s.change_given, 'FM999,999,990.00') || ', cash handed over ' ||
       to_char(COALESCE(SUM(p.amount) FILTER (WHERE p.method = 'cash'), 0), 'FM999,999,990.00') AS detail
FROM sales s LEFT JOIN payments p ON p.sale_id = s.id
WHERE s.change_given > 0 AND s.total >= 0
GROUP BY s.id
HAVING s.change_given > COALESCE(SUM(p.amount) FILTER (WHERE p.method = 'cash'), 0) + 0.01
ORDER BY s.created_at DESC`,
	},
	{
		key:   "archived_locker_money",
		title: "Archived lockers still holding money",
		explain: "Before karots-pos 15f5554, money could still move into an archived locker (the till's default kept " +
			"pre-picking it), so cash sits in a locker that no longer counts in the lockers total. Bring the locker back and move it out.",
		query: `
SELECT l.name AS ref, 'holds ' || to_char(SUM(ll.balance_delta), 'FM999,999,990.00') AS detail
FROM lockers l JOIN locker_ledger ll ON ll.locker_id = l.id
WHERE NOT l.is_active
GROUP BY l.id
HAVING SUM(ll.balance_delta) <> 0
ORDER BY l.name`,
	},
}

// Run runs every check and returns the ones that found something.
func Run(ctx context.Context, q sqlx.QueryerContext) ([]Finding, error) {
	out := []Finding{}
	for _, c := range checks {
		var rows []Row
		if err := sqlx.SelectContext(ctx, q, &rows, c.query); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}
		sample := rows
		if len(sample) > sampleSize {
			sample = sample[:sampleSize]
		}
		out = append(out, Finding{Key: c.key, Title: c.title, Explain: c.explain, Count: len(rows), Sample: sample})
	}
	return out, nil
}
