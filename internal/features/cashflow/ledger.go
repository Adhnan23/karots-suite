package cashflow

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"

	"github.com/shopspring/decimal"
)

// LedgerRow is one line of the unified cash-flow ledger — a locker ledger entry
// or a till cash movement, normalized to a single shape. Delta is signed from the
// location's perspective: positive = cash in, negative = cash out. Internal marks
// one side of a move between two of the shop's own places (locker↔locker,
// till↔locker): real for that place, but the shop as a whole gained or lost
// nothing. Place is the MoneySource value ("locker:3", "till:<user id>");
// ReceiptID is the CR- receipt its move wrote (nil when none matched).
type LedgerRow struct {
	CreatedAt time.Time       `db:"created_at"`
	Place     string          `db:"place"`
	Location  string          `db:"location"`
	Kind      string          `db:"kind"`
	Delta     decimal.Decimal `db:"delta"`
	Note      string          `db:"note"`
	Internal  bool            `db:"internal"`
	ReceiptID *int64          `db:"receipt_id"`
	ReceiptNo *string         `db:"receipt_no"`
}

// ledgerUnion is every tracked money event in [$1, $2). A
// till movement is internal when a locker leg of the same transaction (same
// now(), same session, opposite amount) names its till session as the
// counterparty. ref_kind / ref_id / src let the page query find each row's receipt.
// ponytail: a till→till move has no locker leg, so its two sides count as out +
// in (they cancel in the net); add a counter-session column if that ever matters.
const ledgerUnion = `
	SELECT ll.created_at, 'locker:' || ll.locker_id AS place, l.name AS location, ll.kind, ll.balance_delta AS delta, ll.note,
	       COALESCE(ll.counterparty IN ('till','locker'), false) AS internal, ll.ref_kind, ll.ref_id, 'locker' AS src
	FROM locker_ledger ll
	JOIN lockers l ON l.id = ll.locker_id
	WHERE ll.created_at >= $1 AND ll.created_at < $2
	UNION ALL
	SELECT cm.created_at, 'till:' || cm.user_id, 'Till — ' || u.name, cm.type::text, cm.amount, COALESCE(cm.reason, ''),
	       EXISTS (SELECT 1 FROM locker_ledger ll2
	               WHERE ll2.counter_till_session = cm.session_id
	                 AND ll2.created_at = cm.created_at
	                 AND ll2.balance_delta = -cm.amount),
	       NULL, NULL, 'till'
	FROM cash_movements cm
	JOIN users u ON u.id = cm.user_id
	WHERE cm.created_at >= $1 AND cm.created_at < $2
	  AND cm.type IN ('withdrawal','pay_in','credit_payment','refund','expense')`

// LedgerFilter narrows the ledger page: Place is a MoneySource value, Kind a
// locker_ledger kind or cash_movements type ("" = any). Limit <= 0 = the whole
// range (the printable sheet).
type LedgerFilter struct {
	Place  string
	Kind   string
	Limit  int
	Offset int
}

// LedgerPage is one page of the ledger over [from, to), newest first, each row
// with the CR- receipt its move wrote: same tx time and amount, and — for a
// locker leg — the same ref.
func LedgerPage(ctx context.Context, q db.Queryer, from, to time.Time, f LedgerFilter) ([]LedgerRow, error) {
	var rows []LedgerRow
	var lim any // NULL = no limit
	if f.Limit > 0 {
		lim = f.Limit
	}
	err := q.SelectContext(ctx, &rows, `
		SELECT x.created_at, x.place, x.location, x.kind, x.delta, x.note, x.internal, mr.id AS receipt_id, mr.receipt_no
		FROM (`+ledgerUnion+`) x
		LEFT JOIN LATERAL (
			SELECT id, receipt_no FROM money_receipts m
			WHERE m.created_at = x.created_at AND m.amount = abs(x.delta)
			  AND (x.src = 'till' OR (m.ref_kind IS NOT DISTINCT FROM x.ref_kind AND m.ref_id IS NOT DISTINCT FROM x.ref_id))
			ORDER BY m.id LIMIT 1
		) mr ON true
		WHERE ($5 = '' OR x.place = $5) AND ($6 = '' OR x.kind = $6)
		ORDER BY x.created_at DESC, x.place LIMIT $3 OFFSET $4`, from, to, lim, max(f.Offset, 0), f.Place, f.Kind)
	if err != nil {
		return nil, apperr.Internal("failed to load cash-flow ledger", err)
	}
	return rows, nil
}

// Totals is the cash-flow summary of [from, to), summed in SQL over the whole
// range — never over one page of rows.
type Totals struct {
	Rows      int             `db:"rows"` // ledger rows in range
	TillSales decimal.Decimal // cash taken for sales at the tills, net of change
	In        decimal.Decimal `db:"in"`  // other money in from outside (credit collected, card/online takings, interest, …)
	Openings  decimal.Decimal `db:"openings"` // opening balances of lockers made in range: money already there, not money in
	Out       decimal.Decimal `db:"out"` // money that left the shop (expenses, suppliers, refunds, …)
	Moved     decimal.Decimal // moved between the shop's own places, each move counted once
	Net       decimal.Decimal // TillSales + In + Openings − Out
}

// RangeTotals: opening balances are their own line (money a locker already
// held when it was added, not money that came in), and internal moves are left
// out of In/Out (a transfer is not money
// gained once and lost once). Till cash sales never appear as ledger rows (they
// reach a locker as an internal move when the till closes), so they're counted
// from the payments themselves, the same way the drawer does.
func RangeTotals(ctx context.Context, q db.Queryer, from, to time.Time) (Totals, error) {
	var t Totals
	err := q.GetContext(ctx, &t, `
		SELECT count(*) AS "rows",
		       COALESCE(SUM(GREATEST(delta,0)) FILTER (WHERE NOT internal AND kind <> 'open_balance'), 0) AS "in",
		       COALESCE(SUM(delta) FILTER (WHERE kind = 'open_balance'), 0) AS "openings",
		       COALESCE(SUM(GREATEST(-delta,0)) FILTER (WHERE NOT internal AND kind <> 'open_balance'), 0) AS "out"
		FROM (`+ledgerUnion+`) x`, from, to)
	if err != nil {
		return t, apperr.Internal("failed to total cash flow", err)
	}
	// Each till↔locker move has exactly one locker leg; a locker↔locker move has
	// two, so only its receiving side counts.
	err = q.GetContext(ctx, &t.Moved, `
		SELECT COALESCE(SUM(CASE WHEN counterparty = 'till' THEN ABS(balance_delta)
		                         WHEN counterparty = 'locker' AND balance_delta > 0 THEN balance_delta
		                         ELSE 0 END), 0)
		FROM locker_ledger WHERE created_at >= $1 AND created_at < $2`, from, to)
	if err != nil {
		return t, apperr.Internal("failed to total cash flow", err)
	}
	err = q.GetContext(ctx, &t.TillSales, `
		SELECT COALESCE((SELECT SUM(p.amount) FROM payments p JOIN sales s ON s.id = p.sale_id
		                 WHERE p.method = 'cash' AND s.created_at >= $1 AND s.created_at < $2), 0)
		     - COALESCE((SELECT SUM(change_given) FROM sales
		                 WHERE created_at >= $1 AND created_at < $2), 0)`, from, to)
	if err != nil {
		return t, apperr.Internal("failed to total cash flow", err)
	}
	t.Net = t.TillSales.Add(t.In).Add(t.Openings).Sub(t.Out)
	return t, nil
}
