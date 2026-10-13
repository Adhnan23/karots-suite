// Package plugintest holds helpers for plugin tests (imported from _test files only).
package plugintest

import (
	"context"
	"io/fs"
	"os"
	"testing"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// MergeTx migrates a plugin onto the dev DB and opens a transaction holding two
// fresh products, from (the duplicate) and into (the one kept), for testing the
// plugin's ProductMerger. Everything is rolled back when the test ends. Skipped
// unless DATABASE_URL is set.
func MergeTx(t *testing.T, migrations fs.FS, versionTable string) (tx *sqlx.Tx, from, into int64) {
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
	if err := appdb.RunMigrationsTable(db.DB, migrations, versionTable); err != nil {
		t.Fatal(err)
	}
	tx, err = db.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.Rollback() }) //nolint:errcheck
	for _, p := range []*int64{&from, &into} {
		if err := tx.Get(p, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ('merge test', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 10, 20) RETURNING id`); err != nil {
			t.Fatal(err)
		}
	}
	return tx, from, into
}

// Count runs a COUNT(*)-style query in the transaction.
func Count(t *testing.T, tx *sqlx.Tx, q string, args ...any) int {
	t.Helper()
	var n int
	if err := tx.Get(&n, q, args...); err != nil {
		t.Fatal(err)
	}
	return n
}
