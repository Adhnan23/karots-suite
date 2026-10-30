package pocketcashier

import (
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/plugins/pocketcashier/migrations"

	"github.com/jmoiron/sqlx"
)

// testDB migrates the plugin onto the dev DB.
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
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_pocketcashier"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrationsApply(t *testing.T) {
	db := testDB(t)
	dev := fmt.Sprintf("T-dev-%d", time.Now().UnixNano())
	var id int64
	if err := db.Get(&id, `INSERT INTO pocket_phones (name, device, token_hash) VALUES ('T phone', $1, $2) RETURNING id`, dev, []byte(dev)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec(`DELETE FROM pocket_phones WHERE id = $1`, id) }) //nolint:errcheck
	var status string
	if err := db.Get(&status, `SELECT status FROM pocket_phones WHERE id = $1`, id); err != nil || status != "pending" {
		t.Fatalf("status %q (%v)", status, err)
	}
}
