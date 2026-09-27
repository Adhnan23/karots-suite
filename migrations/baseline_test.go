package migrations_test

import (
	"os"
	"regexp"
	"testing"

	appdb "karots-suite/internal/db"
	"karots-suite/migrations"
)

// Each migrated database must get its OWN random install id. If the squash ever
// froze the dev DB's id into the baseline, every shop would share one support PIN.
func TestBaselineGeneratesFreshInstallID(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := appdb.RunMigrations(conn.DB, migrations.FS); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := conn.Get(&id, `SELECT install_id FROM settings LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9A-F]{8}$`).MatchString(id) {
		t.Fatalf("install_id %q is not 8 upper-hex", id)
	}
	frozen, _ := os.ReadFile("00001_baseline.sql")
	if regexp.MustCompile(`'`+id+`'`).Match(frozen) {
		t.Fatalf("install_id %s is hard-coded in the baseline", id)
	}
}
