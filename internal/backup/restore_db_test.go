package backup

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
)

// TestRestoreAsNonSuperuser proves restore works for an installed shop, whose
// role owns the database but is not a superuser (install.sh / install.ps1), and
// that rows load in any order (child before parent, self-reference). Skipped
// unless DATABASE_URL is set; needs a role that can create roles and databases.
func TestRestoreAsNonSuperuser(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	cleanup := func() {
		admin.Exec(`DROP DATABASE IF EXISTS restoretest_db WITH (FORCE)`)
		admin.Exec(`DROP ROLE IF EXISTS restoretest_owner`)
	}
	cleanup()
	defer cleanup()
	for _, q := range []string{
		`CREATE ROLE restoretest_owner LOGIN PASSWORD 'pw'`,
		`CREATE DATABASE restoretest_db OWNER restoretest_owner`,
	} {
		if _, err := admin.Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	u, _ := url.Parse(dsn)
	u.User = url.UserPassword("restoretest_owner", "pw")
	u.Path = "/restoretest_db"
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE cats (id serial PRIMARY KEY, parent_id int REFERENCES cats(id))`,
		`CREATE TABLE items (id serial PRIMARY KEY, cat_id int NOT NULL REFERENCES cats(id))`,
		// Mirrors trg_product_stock: an insert trigger whose rows the archive already holds.
		`CREATE TABLE cat_stats (cat_id int PRIMARY KEY REFERENCES cats(id))`,
		`CREATE FUNCTION add_stats() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO cat_stats VALUES (NEW.id); RETURN NEW; END $$`,
		`CREATE TRIGGER trg_stats AFTER INSERT ON cats FOR EACH ROW EXECUTE FUNCTION add_stats()`,
		`INSERT INTO cats (id, parent_id) VALUES (1, NULL), (2, 1)`,
		`INSERT INTO items (cat_id) VALUES (2)`,
		// Parent row comes after its child in the dump.
		`UPDATE cats SET parent_id = 2 WHERE id = 1`,
		`UPDATE cats SET parent_id = NULL WHERE id = 2`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	var buf bytes.Buffer
	if err := Dump(ctx, db, "test", &buf, DumpOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := Restore(ctx, db, &buf); err != nil {
		t.Fatalf("restore as non-superuser: %v", err)
	}

	var n, fks int
	var trigOn bool
	db.Get(&trigOn, `SELECT tgenabled = 'O' FROM pg_trigger WHERE tgname = 'trg_stats'`)
	if !trigOn {
		t.Fatal("trigger left disabled after restore")
	}
	db.Get(&n, `SELECT count(*) FROM items JOIN cats ON cats.id = items.cat_id`)
	db.Get(&fks, `SELECT count(*) FROM pg_constraint WHERE contype = 'f'`)
	if n != 1 || fks != 3 {
		t.Fatalf("after restore: joined items=%d, foreign keys=%d; want 1 and 3", n, fks)
	}
}
