package datacheck

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

func counts(t *testing.T, ctx context.Context, tx *sqlx.Tx) map[string]int {
	t.Helper()
	f, err := Run(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]int{}
	for _, x := range f {
		m[x.Key] = x.Count
		if len(x.Sample) == 0 || x.Sample[0].Ref == "" || x.Sample[0].Detail == "" {
			t.Errorf("%s: empty sample %+v", x.Key, x.Sample)
		}
	}
	return m
}

// Each check finds exactly the bad shape its karots-pos fix stopped writing,
// and a correct row next to it is not reported.
func TestChecksFindOldDamage(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	cases := []struct {
		key  string
		bad  string // inserts one bad shape
		good string // a correct neighbour that must not count
	}{
		{"over_credited_returns",
			// 1000 sale, 400 paid → 600 owed; two returns took 400 + 400.
			`WITH s AS (INSERT INTO sales (receipt_no, customer_id, total, paid_amount, cashier_id, status)
			   VALUES ('T-OC1', (SELECT id FROM c), 1000, 400, (SELECT id FROM users LIMIT 1), 'credit') RETURNING id)
			 INSERT INTO sale_returns (sale_id, credit_reduction, created_by)
			 SELECT s.id, v, (SELECT id FROM users LIMIT 1) FROM s, (VALUES (400), (400)) AS r(v)`,
			`WITH s AS (INSERT INTO sales (receipt_no, customer_id, total, paid_amount, cashier_id, status)
			   VALUES ('T-OC2', (SELECT id FROM c), 1000, 400, (SELECT id FROM users LIMIT 1), 'credit') RETURNING id)
			 INSERT INTO sale_returns (sale_id, credit_reduction, created_by)
			 SELECT s.id, v, (SELECT id FROM users LIMIT 1) FROM s, (VALUES (400), (200)) AS r(v)`},
		{"refund_over_paid",
			`WITH s AS (INSERT INTO sales (receipt_no, total, paid_amount, cashier_id, status)
			   VALUES ('T-RP1', 360, 360, (SELECT id FROM users LIMIT 1), 'returned') RETURNING id)
			 INSERT INTO sale_returns (sale_id, refund_amount, created_by) SELECT id, 370, (SELECT id FROM users LIMIT 1) FROM s`,
			`WITH s AS (INSERT INTO sales (receipt_no, total, paid_amount, cashier_id, status)
			   VALUES ('T-RP2', 360, 360, (SELECT id FROM users LIMIT 1), 'returned') RETURNING id)
			 INSERT INTO sale_returns (sale_id, refund_amount, created_by) SELECT id, 360, (SELECT id FROM users LIMIT 1) FROM s`},
		{"returned_without_record",
			`INSERT INTO sales (receipt_no, total, cashier_id, status) VALUES ('T-RW1', 50, (SELECT id FROM users LIMIT 1), 'returned')`,
			`INSERT INTO sales (receipt_no, total, cashier_id, status) VALUES ('T-RW2', 50, (SELECT id FROM users LIMIT 1), 'completed')`},
		{"deleted_expense_money",
			`INSERT INTO locker_ledger (locker_id, balance_delta, kind, counterparty, ref_kind, ref_id)
			 VALUES ((SELECT id FROM l), -500, 'payment', 'external', 'expense', 999999999)`,
			// A voided expense (deleted, money put back under the same ref) is fine.
			`INSERT INTO locker_ledger (locker_id, balance_delta, kind, counterparty, ref_kind, ref_id)
			 SELECT (SELECT id FROM l), v, 'payment', 'external', 'expense', 999999998 FROM (VALUES (-300), (300)) AS r(v)`},
		{"change_beyond_cash",
			`WITH s AS (INSERT INTO sales (receipt_no, total, paid_amount, change_given, cashier_id)
			   VALUES ('T-CB1', 200, 5000, 4800, (SELECT id FROM users LIMIT 1)) RETURNING id)
			 INSERT INTO payments (sale_id, method, amount) SELECT id, 'card', 5000 FROM s`,
			`WITH s AS (INSERT INTO sales (receipt_no, total, paid_amount, change_given, cashier_id)
			   VALUES ('T-CB2', 200, 5000, 4800, (SELECT id FROM users LIMIT 1)) RETURNING id)
			 INSERT INTO payments (sale_id, method, amount) SELECT id, 'cash', 5000 FROM s`},
		{"archived_locker_money",
			`WITH a AS (INSERT INTO lockers (name, is_active) VALUES ('T archived full', false) RETURNING id)
			 INSERT INTO locker_ledger (locker_id, balance_delta, kind, counterparty) SELECT id, 750, 'adjust', 'external' FROM a`,
			`WITH a AS (INSERT INTO lockers (name, is_active) VALUES ('T archived empty', false) RETURNING id)
			 INSERT INTO locker_ledger (locker_id, balance_delta, kind, counterparty)
			 SELECT id, v, 'adjust', 'external' FROM a, (VALUES (750), (-750)) AS r(v)`},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			tx, err := db.BeginTxx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback() //nolint:errcheck
			// Fixtures every case may use: a customer (c) and an active locker (l).
			for _, q := range []string{
				`CREATE TEMP TABLE c ON COMMIT DROP AS WITH x AS (INSERT INTO customers (name) VALUES ('T datacheck') RETURNING id) SELECT id FROM x`,
				`CREATE TEMP TABLE l ON COMMIT DROP AS WITH x AS (INSERT INTO lockers (name) VALUES ('T datacheck safe') RETURNING id) SELECT id FROM x`,
			} {
				if _, err := tx.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}
			before := counts(t, ctx, tx)
			for _, q := range []string{tc.bad, tc.good} {
				if _, err := tx.ExecContext(ctx, q); err != nil {
					t.Fatalf("%s\n%v", q, err)
				}
			}
			after := counts(t, ctx, tx)
			for k := range after {
				want := before[k]
				if k == tc.key {
					want++
				}
				if after[k] != want {
					t.Errorf("%s: %d findings, want %d", k, after[k], want)
				}
			}
			if after[tc.key] != before[tc.key]+1 {
				t.Errorf("%s not found", tc.key)
			}
		})
	}
}
