package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	appdb "karots-suite/internal/db"
)

// scratchDB creates an empty database for one test and drops it afterwards.
// Skipped unless DATABASE_URL is set.
func scratchDB(t *testing.T, name string) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		admin.Close()
	})
	return db
}

func gzArchive(t *testing.T, arc Archive) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(arc); err != nil {
		t.Fatal(err)
	}
	gz.Close()
	return &buf
}

func s(v string) *string { return &v }

const compatSchema = `
CREATE TABLE units (id serial PRIMARY KEY, name text NOT NULL);
CREATE TABLE products (id serial PRIMARY KEY, name text NOT NULL, unit_id int REFERENCES units(id),
  is_active boolean NOT NULL DEFAULT true);
CREATE SEQUENCE sales_receipt_seq;
INSERT INTO units (name) VALUES ('old');
INSERT INTO products (name) VALUES ('old product');`

func count(t *testing.T, db *sqlx.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT count(*) FROM `+table); err != nil {
		t.Fatal(err)
	}
	return n
}

// A backup from another schema version (a newer karots-pos with an extra
// column, a plugin shop with tables this build lacks, an older one missing a
// column that has a default) still restores, and the report says what it left out.
func TestRestoreAcrossSchemaDifferences(t *testing.T) {
	db := scratchDB(t, "restorecompat_db")
	if _, err := db.Exec(compatSchema); err != nil {
		t.Fatal(err)
	}
	arc := Archive{Version: FormatVersion, Tables: []Table{
		{Name: "units", Columns: []string{"id", "name", "short_code"},
			Rows: [][]*string{{s("7"), s("kg"), s("KG")}}},
		{Name: "recharge_bills", Columns: []string{"id"}, Rows: [][]*string{{s("1")}}},
		{Name: "products", Columns: []string{"id", "name", "unit_id"},
			Rows: [][]*string{{s("3"), s("Rice"), s("7")}}},
	}}
	rep, err := Restore(context.Background(), db, gzArchive(t, arc))
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(rep.SkippedTables) != 1 || rep.SkippedTables[0] != "recharge_bills" {
		t.Errorf("SkippedTables = %v, want [recharge_bills]", rep.SkippedTables)
	}
	if len(rep.DroppedColumns) != 1 || rep.DroppedColumns[0] != "units.short_code" {
		t.Errorf("DroppedColumns = %v, want [units.short_code]", rep.DroppedColumns)
	}
	if rep.Tables != 2 || rep.Rows != 2 {
		t.Errorf("Tables=%d Rows=%d, want 2 and 2", rep.Tables, rep.Rows)
	}
	var active bool
	var unit int
	if err := db.QueryRow(`SELECT is_active, unit_id FROM products WHERE id = 3`).Scan(&active, &unit); err != nil {
		t.Fatal(err)
	}
	if !active || unit != 7 {
		t.Errorf("product: is_active=%v unit_id=%d, want true and 7", active, unit)
	}
	if count(t, db, "units") != 1 || count(t, db, "products") != 1 {
		t.Error("old rows survived the full restore")
	}
	var next int
	db.Get(&next, `SELECT nextval('products_id_seq')`)
	if next != 4 {
		t.Errorf("products id sequence next = %d, want 4", next)
	}
}

// A bad file never changes the database.
func TestRestoreFailureChangesNothing(t *testing.T) {
	db := scratchDB(t, "restorefail_db")
	if _, err := db.Exec(compatSchema); err != nil {
		t.Fatal(err)
	}
	ok := gzArchive(t, Archive{Version: FormatVersion}).Bytes()
	cases := map[string][]byte{
		"truncated gzip": ok[:len(ok)/2],
		"not gzip":       []byte("hello"),
		"bad json":       gzArchive(t, Archive{}).Bytes()[:0],
		"future version": gzArchive(t, Archive{Version: FormatVersion + 1}).Bytes(),
		"breaks a foreign key": gzArchive(t, Archive{Version: FormatVersion, Tables: []Table{
			{Name: "products", Columns: []string{"id", "name", "unit_id"}, Rows: [][]*string{{s("1"), s("x"), s("99")}}},
		}}).Bytes(),
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Restore(context.Background(), db, bytes.NewReader(file)); err == nil {
				t.Fatal("restore succeeded, want an error")
			}
			var p string
			db.Get(&p, `SELECT name FROM products`)
			if count(t, db, "units") != 1 || count(t, db, "products") != 1 || p != "old product" {
				t.Fatal("a failed restore changed the data")
			}
		})
	}
}

// The S- receipt sequence moves past the restored numbers.
func TestRestoreAdvancesReceiptSequence(t *testing.T) {
	db := scratchDB(t, "restoreseq_db")
	if _, err := db.Exec(compatSchema + `
CREATE TABLE sales (id serial PRIMARY KEY, receipt_no text UNIQUE);`); err != nil {
		t.Fatal(err)
	}
	arc := Archive{Version: FormatVersion, Tables: []Table{
		{Name: "sales", Columns: []string{"id", "receipt_no"}, Rows: [][]*string{{s("1"), s("S-000041")}}},
	}}
	if _, err := Restore(context.Background(), db, gzArchive(t, arc)); err != nil {
		t.Fatal(err)
	}
	var next int64
	db.Get(&next, `SELECT nextval('sales_receipt_seq')`)
	if next <= 41 {
		t.Errorf("sales_receipt_seq next = %d, want > 41", next)
	}
}
