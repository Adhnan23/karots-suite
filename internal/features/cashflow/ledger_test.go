package cashflow

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"karots-suite/internal/db"

	"github.com/shopspring/decimal"
)

// Cash-flow totals are summed over the whole range (karots-pos summed the newest
// 500 rows), and a move between two of the shop's own places is counted once as
// "moved" — not once as money in and again as money out. Runs in a rolled-back
// tx on a far-future range so no real data is in it.
func TestRangeTotalsWholeRangeAndInternalMoves(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	day := time.Date(2099, 3, 1, 10, 0, 0, 0, time.Local)
	var a, b, user, sess int64
	must(tx.GetContext(ctx, &a, `INSERT INTO lockers (name, kind) VALUES ('TEST cf A', 'safe') RETURNING id`))
	must(tx.GetContext(ctx, &b, `INSERT INTO lockers (name, kind) VALUES ('TEST cf B', 'safe') RETURNING id`))
	must(tx.GetContext(ctx, &user, `SELECT id FROM users ORDER BY id LIMIT 1`))
	must(tx.GetContext(ctx, &sess, `INSERT INTO cash_register (user_id, opened_at, closed_at) VALUES ($1, $2, $2) RETURNING id`, user, day))

	leg := func(locker int64, delta, cp string, at time.Time, counterLocker, counterSess any) {
		t.Helper()
		_, err := tx.ExecContext(ctx, `INSERT INTO locker_ledger
			(locker_id, balance_delta, kind, counterparty, counter_locker_id, counter_till_session, created_at)
			VALUES ($1, $2, 'adjust', $3, $4, $5, $6)`, locker, delta, cp, counterLocker, counterSess, at)
		must(err)
	}
	move := func(mtype, amount string, at time.Time) {
		t.Helper()
		_, err := tx.ExecContext(ctx, `INSERT INTO cash_movements (session_id, user_id, type, amount, created_at)
			VALUES ($1, $2, $3, $4, $5)`, sess, user, mtype, amount, at)
		must(err)
	}

	from, to := time.Date(2099, 3, 1, 0, 0, 0, 0, time.Local), time.Date(2099, 3, 2, 0, 0, 0, 0, time.Local)
	leg(a, "1000", "external", day, nil, nil)                    // money in
	if _, err := tx.ExecContext(ctx, `INSERT INTO locker_ledger (locker_id, balance_delta, kind, created_at) VALUES ($1, 400, 'open_balance', $2)`, b, day); err != nil {
		t.Fatal(err) // an opening balance: its own line, not "money in"
	}
	leg(a, "-300", "locker", day.Add(time.Minute), b, nil)       // A → B: moved, not in/out
	leg(b, "300", "locker", day.Add(time.Minute), a, nil)        //
	leg(a, "-100", "external", day.Add(2*time.Minute), nil, nil) // money out
	at := day.Add(3 * time.Minute)                               // till → A withdrawal, one tx
	move("withdrawal", "-200", at)
	leg(a, "200", "till", at, nil, sess)
	move("expense", "-50", day.Add(4*time.Minute)) // paid from the drawer: out
	for i := range 60 {                            // enough rows to need a second page
		leg(b, "1", "external", day.Add(time.Duration(10+i)*time.Minute), nil, nil)
	}
	// Range edges: the last second of the day is in, the next midnight is out.
	leg(b, "7", "external", to.Add(-time.Second), nil, nil)
	leg(b, "1000", "external", to, nil, nil)
	leg(b, "1000", "external", from.Add(-time.Second), nil, nil)

	tot, err := RangeTotals(ctx, tx, from, to)
	must(err)
	want := func(name string, got decimal.Decimal, w int64) {
		t.Helper()
		if !got.Equal(decimal.NewFromInt(w)) {
			t.Errorf("%s = %s, want %d", name, got, w)
		}
	}
	want("In", tot.In, 1067)
	want("Out", tot.Out, 150)
	want("Moved", tot.Moved, 500)
	want("Openings", tot.Openings, 400)
	want("Net", tot.Net, 1067+400-150)
	if tot.Rows != 69 {
		t.Errorf("Rows = %d, want 69", tot.Rows)
	}

	// Paging through the ledger visits every row once, and the outside rows add
	// up to the same totals.
	in, out, seen := decimal.Zero, decimal.Zero, 0
	for off := 0; ; off += 25 {
		page, err := LedgerPage(ctx, tx, from, to, LedgerFilter{Limit: 25, Offset: off})
		must(err)
		if len(page) == 0 {
			break
		}
		seen += len(page)
		for _, r := range page {
			if r.Internal || r.Kind == "open_balance" {
				continue
			}
			if r.Delta.IsPositive() {
				in = in.Add(r.Delta)
			} else {
				out = out.Sub(r.Delta)
			}
		}
	}
	if seen != 69 || !in.Equal(tot.In) || !out.Equal(tot.Out) {
		t.Errorf("paged: %d rows, in %s, out %s; want 69, %s, %s", seen, in, out, tot.In, tot.Out)
	}

	// The place and kind filters; an empty range is zeros, not an error.
	rows, err := LedgerPage(ctx, tx, from, to, LedgerFilter{Place: "locker:" + strconv.FormatInt(a, 10)})
	must(err)
	if len(rows) != 4 {
		t.Errorf("locker A rows = %d, want 4", len(rows))
	}
	rows, err = LedgerPage(ctx, tx, from, to, LedgerFilter{Kind: "expense"})
	must(err)
	if len(rows) != 1 || rows[0].Place != "till:"+strconv.FormatInt(user, 10) {
		t.Errorf("expense rows = %+v", rows)
	}
	empty, err := RangeTotals(ctx, tx, from.AddDate(0, 0, 5), to.AddDate(0, 0, 5))
	must(err)
	if empty.Rows != 0 || !empty.Net.IsZero() {
		t.Errorf("empty range = %+v", empty)
	}
}
