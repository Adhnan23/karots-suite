package pharmacy

import (
	"os"
	"testing"

	appdb "karots-suite/internal/db"
	altmigrations "karots-suite/plugins/alternatives/migrations"
	"karots-suite/plugins/pharmacy/migrations"

	"github.com/jmoiron/sqlx"
)

// testDB is the dev database with Alternatives' tables (0007 points at them) and
// pharmacy's migrated; skipped without DATABASE_URL.
func testDB(t *testing.T) *sqlx.DB {
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
	if err := appdb.RunMigrationsTable(db.DB, altmigrations.FS, "goose_db_version_alternatives"); err != nil {
		t.Fatal(err)
	}
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_pharmacy"); err != nil {
		t.Fatal(err)
	}
	return db
}
