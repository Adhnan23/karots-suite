package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
)

func txDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	d, err := Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// A joined transaction inside another joined one: the inner one failing and then
// the outer one failing undoes everything the outer one did, not only the inner part.
func TestJoinedNestedRollbackUndoesTheOuter(t *testing.T) {
	d := txDB(t)
	ctx := context.Background()
	boom := errors.New("boom")
	var n int
	err := WithTx(ctx, d, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`CREATE TEMP TABLE tx_nest (v int) ON COMMIT DROP`); err != nil {
			return err
		}
		c := InTx(ctx, tx)
		_ = WithTx(c, d, func(tx *sqlx.Tx) error { // the line
			tx.MustExec(`INSERT INTO tx_nest VALUES (1)`)
			return WithTx(c, d, func(tx *sqlx.Tx) error { // the write inside it
				tx.MustExec(`INSERT INTO tx_nest VALUES (2)`)
				return boom
			})
		})
		return tx.Get(&n, `SELECT count(*) FROM tx_nest`)
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the outer savepoint's row survived its rollback: %d rows", n)
	}
}

// A joined fn that swallows a failed statement leaves its caller's transaction
// usable: the savepoint is undone and the failure reported.
func TestJoinedSwallowedErrorLeavesTheCallerUsable(t *testing.T) {
	d := txDB(t)
	ctx := context.Background()
	err := WithTx(ctx, d, func(tx *sqlx.Tx) error {
		c := InTx(ctx, tx)
		jerr := WithTx(c, d, func(tx *sqlx.Tx) error {
			_, _ = tx.Exec(`SELECT 1/0`) // swallowed
			return nil
		})
		if jerr == nil {
			t.Error("a swallowed failure was not reported")
		}
		var one int
		return tx.Get(&one, `SELECT 1`) // the caller goes on
	})
	if err != nil {
		t.Fatalf("the caller's transaction stayed aborted: %v", err)
	}
}
