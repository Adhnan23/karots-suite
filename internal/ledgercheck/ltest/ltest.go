// Package ltest is the test side of the auditor: a throwaway migrated
// database of its own, and Check, which fails a test with every broken rule.
package ltest

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/db"
	"karots-suite/internal/ledgercheck"
)

// NewDB makes an empty, migrated database on the DATABASE_URL server (never
// suite_db itself, so it can run beside Playwright) and drops it after the test.
func NewDB(t testing.TB) *sqlx.DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := db.Connect(base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("ledger_%d", rand.Uint32())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	admin.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`) //nolint:errcheck
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	sh, err := db.Connect(base[:i+1] + name + q)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sh.Close()
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
		admin.Close()
	})
	if err := app.Migrate(sh); err != nil {
		t.Fatal(err)
	}
	if err := app.EnsureSystemAdmin(sh, ""); err != nil {
		t.Fatal(err)
	}
	return sh
}

// Check runs the auditor and fails the test with every violation.
func Check(t testing.TB, ctx context.Context, q sqlx.QueryerContext) {
	t.Helper()
	vs, err := ledgercheck.Run(ctx, q)
	if err != nil {
		t.Fatalf("ledgercheck: %v", err)
	}
	for _, v := range vs {
		t.Errorf("%s %s: %s", v.Rule, v.Ref, v.Detail)
	}
}
