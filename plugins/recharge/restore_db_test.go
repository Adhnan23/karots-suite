package recharge

import (
	"net/url"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/recharge/migrations"

	"github.com/jmoiron/sqlx"
)

// freshDB is a fresh database with core and recharge migrated.
func freshDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	const name = "recharge_test_db"
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) }) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, m := range []error{
		appdb.RunMigrations(db.DB, coremigrations.FS),
		appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_recharge"),
	} {
		if m != nil {
			t.Fatal(m)
		}
	}
	return db
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
