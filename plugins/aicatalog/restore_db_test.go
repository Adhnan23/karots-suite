package aicatalog

import (
	"net/url"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/aicatalog/migrations"

	"github.com/jmoiron/sqlx"
)

func freshDB(t *testing.T, name string) *sqlx.DB {
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
	if err := appdb.RunMigrations(db.DB, coremigrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_aicatalog"); err != nil {
		t.Fatal(err)
	}
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
