// Package ltest is the test side of the auditor: a throwaway migrated
// database of its own, and Check, which fails a test with every broken rule.
package ltest

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/db"
	"karots-suite/internal/ledgercheck"
	"karots-suite/migrations"
)

// NewDB makes an empty, migrated database on the DATABASE_URL server (never
// suite_db itself, so it can run beside Playwright) and drops it after the test.
// It is a copy of a template migrated once per set of migrations (named by
// their hash, kept on the server), so a fresh shop costs a file copy, not 38
// migrations.
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
	tpl, err := template(admin, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("ledger_%d", rand.Uint32())
	if _, err := admin.Exec(`CREATE DATABASE ` + name + ` TEMPLATE ` + tpl); err != nil {
		t.Fatal(err)
	}
	sh, err := db.Connect(dsnFor(base, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sh.Close()
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
		admin.Close()
	})
	return sh
}

var tplOnce struct {
	sync.Mutex
	name string
}

// template returns the migrated template's name, making it the first time.
func template(admin *sqlx.DB, base string) (string, error) {
	tplOnce.Lock()
	defer tplOnce.Unlock()
	if tplOnce.name != "" {
		return tplOnce.name, nil
	}
	h := sha256.New()
	if err := fs.WalkDir(migrations.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(migrations.FS, p)
		h.Write([]byte(p))
		h.Write(b)
		return err
	}); err != nil {
		return "", err
	}
	name := fmt.Sprintf("ledger_tpl_%x", h.Sum(nil)[:6])
	var ok bool
	if err := admin.Get(&ok, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name); err != nil {
		return "", err
	}
	if !ok {
		// Built under another name, then renamed: a half-made template (a run
		// killed mid-migration) is never mistaken for a ready one.
		work := fmt.Sprintf("%s_wip_%d", name, rand.Uint32())
		if _, err := admin.Exec(`CREATE DATABASE ` + work); err != nil {
			return "", err
		}
		admin.Exec(`ALTER DATABASE ` + work + ` SET synchronous_commit = off`) //nolint:errcheck
		w, err := db.Connect(dsnFor(base, work))
		if err != nil {
			return "", err
		}
		err = app.Migrate(w)
		if err == nil {
			err = app.EnsureSystemAdmin(w, "")
		}
		w.Close()
		if err == nil {
			_, err = admin.Exec(`ALTER DATABASE ` + work + ` RENAME TO ` + name)
		}
		if err != nil {
			admin.Exec(`DROP DATABASE IF EXISTS ` + work + ` WITH (FORCE)`) //nolint:errcheck
			// Another test process made it first: use theirs.
			if exists := admin.Get(&ok, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name); exists != nil || !ok {
				return "", err
			}
		}
	}
	tplOnce.name = name
	return name, nil
}

func dsnFor(base, name string) string {
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	return base[:i+1] + name + q
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
