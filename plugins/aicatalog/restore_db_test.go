package aicatalog

import (
	"karots-suite/internal/ledgercheck/ltest"
	"testing"

	"github.com/jmoiron/sqlx"
)

// freshDB is a test's own database: a copy of a template migrated once (core and
// this plugin), not 40-odd migrations per test. name is kept for the callers' sake.
//
// ponytail: tests stay one at a time — each Setup adds hooks to core's process-wide
// lists (a race side by side); a per-test registry would let them run in parallel.
func freshDB(t *testing.T, _ string) *sqlx.DB {
	t.Helper()
	db := ltest.NewDB(t)
	mustExec(t, db, nil, `INSERT INTO categories (name) VALUES ('Old shop')`)
	return db
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

func count(t *testing.T, db *sqlx.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Get(&n, q, args...); err != nil {
		t.Fatal(err)
	}
	return n
}
