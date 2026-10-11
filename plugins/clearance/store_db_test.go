package clearance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/plugins/clearance/migrations"

	"github.com/jmoiron/sqlx"
)

// testStore migrates clearance onto the dev DB and makes one product of its own:
// in stock (qty), never sold, with one lot received two days ago.
func testStore(t *testing.T, qty string) (*Store, *sqlx.DB, int64) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_clearance"); err != nil {
		t.Fatal(err)
	}
	var pid int64
	mustExec(t, db, &pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 100, 200) RETURNING id`,
		fmt.Sprintf("Clr test %d", time.Now().UnixNano()))
	mustExec(t, db, nil, `INSERT INTO stock (product_id, quantity) VALUES ($1, $2) ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, pid, qty)
	mustExec(t, db, nil, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, created_at)
		VALUES ($1, $2, $2, 100, now() - interval '2 days')`, pid, qty)
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM clearance_markdowns WHERE product_id = $1`, `DELETE FROM stock_batches WHERE product_id = $1`,
			`DELETE FROM stock WHERE product_id = $1`, `DELETE FROM products WHERE id = $1`} {
			db.Exec(q, pid) //nolint:errcheck
		}
	})
	return NewStore(db), db, pid
}

func mustExec(t *testing.T, db *sqlx.DB, dest *int64, q string, args ...any) {
	t.Helper()
	var err error
	if dest != nil {
		err = db.Get(dest, q, args...)
	} else {
		_, err = db.Exec(q, args...)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func candidate(t *testing.T, st *Store, pid int64) *Candidate {
	t.Helper()
	rows, err := st.Candidates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if rows[i].ProductID == pid {
			return &rows[i]
		}
	}
	return nil
}

func isLive(t *testing.T, st *Store, pid int64) bool {
	t.Helper()
	m, err := st.live(context.Background(), []int64{pid})
	if err != nil {
		t.Fatal(err)
	}
	_, ok := m[pid]
	return ok
}

// An approved markdown is live only for the stock it was approved for: it ends
// when that stock sells out or fresh stock arrives, and the product is an
// ordinary candidate again until someone approves it again.
func TestMarkdownEndsItself(t *testing.T) {
	st, db, pid := testStore(t, "5")
	ctx := context.Background()
	if err := st.Approve(ctx, pid, dec("20"), 1); err != nil {
		t.Fatal(err)
	}
	if !isLive(t, st, pid) || candidate(t, st, pid).Status != "live" {
		t.Fatal("approved markdown is not live")
	}
	mustExec(t, db, nil, `UPDATE stock SET quantity = 0 WHERE product_id = $1`, pid)
	if isLive(t, st, pid) {
		t.Fatal("still live after selling out")
	}
	mustExec(t, db, nil, `UPDATE stock SET quantity = 6 WHERE product_id = $1`, pid)
	mustExec(t, db, nil, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price) VALUES ($1, 6, 6, 110)`, pid)
	if isLive(t, st, pid) {
		t.Fatal("still live on fresh stock")
	}
	if c := candidate(t, st, pid); c == nil || c.Status != "" {
		t.Fatalf("after fresh stock want a plain candidate, got %+v", c)
	}
	if err := st.Approve(ctx, pid, dec("10"), 1); err != nil {
		t.Fatal(err)
	}
	if !isLive(t, st, pid) {
		t.Fatal("re-approved markdown is not live")
	}
}

// Dismiss hides a product for one stale period, not forever.
func TestSnoozeComesBack(t *testing.T) {
	st, db, pid := testStore(t, "3")
	if err := st.Snooze(context.Background(), pid, 1); err != nil {
		t.Fatal(err)
	}
	if c := candidate(t, st, pid); c == nil || c.Status != "snoozed" || c.SnoozedUntil == nil {
		t.Fatalf("want snoozed, got %+v", c)
	}
	mustExec(t, db, nil, `UPDATE clearance_markdowns SET updated_at = now() - interval '61 days' WHERE product_id = $1`, pid)
	if c := candidate(t, st, pid); c == nil || c.Status != "" {
		t.Fatalf("want a candidate again after the stale period, got %+v", c)
	}
	if err := st.Snooze(context.Background(), pid, 1); err != nil {
		t.Fatal(err)
	}
	if err := st.Unsnooze(context.Background(), pid); err != nil {
		t.Fatal(err)
	}
	if c := candidate(t, st, pid); c == nil || c.Status != "" {
		t.Fatalf("unsnooze: got %+v", c)
	}
}

// An expiring lot floors at its own cost, not the product's latest cost.
func TestExpiringLotCost(t *testing.T) {
	st, db, pid := testStore(t, "4")
	mustExec(t, db, nil, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, expiry_date)
		VALUES ($1, 2, 2, 80, CURRENT_DATE + 5)`, pid)
	c := candidate(t, st, pid)
	if c == nil || c.SoonestExpiry == nil || c.LotCost == nil || !c.LotCost.Equal(dec("80")) {
		t.Fatalf("want the expiring lot's cost 80, got %+v", c)
	}
}
