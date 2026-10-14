package pharmacy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"karots-suite/internal/backup"
	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	altmigrations "karots-suite/plugins/alternatives/migrations"
	"karots-suite/plugins/pharmacy/migrations"

	"github.com/jmoiron/sqlx"
)

// freshDB is a fresh database with core, Alternatives and pharmacy migrated.
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
	const name = "pharmacy_test_db"
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
		appdb.RunMigrationsTable(db.DB, altmigrations.FS, "goose_db_version_alternatives"),
		appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_pharmacy"),
	} {
		if m != nil {
			t.Fatal(m)
		}
	}
	return db
}

// A karots-pos pharmacy backup (its 0001–0006 columns exactly) restores with no
// mapping: patients, scripts with per-drug refill rows, dispenses and the old
// hand-kept controlled register all come back.
func TestKarotsPosPharmacyRestore(t *testing.T) {
	db := freshDB(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var cust, prod int64
	must(db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Medicines') RETURNING id`))
	must(db.Get(&cust, `INSERT INTO customers (name, phone) VALUES ('Nimal Perera', '0771234567') RETURNING id`))
	must(db.Get(&prod, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('Panadol 500mg', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 2, 3) RETURNING id`))

	ts := time.Now().Add(-20 * 24 * time.Hour).UTC().Format(time.RFC3339)
	day := time.Now().Add(-20 * 24 * time.Hour).UTC().Format("2006-01-02")
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"pharmacy"}, Tables: []backup.Table{
		{Name: "pharmacy_patients", Columns: []string{"id", "customer_id", "dob", "sex", "nic", "allergies", "notes", "created_at"},
			Rows: [][]*string{{s("1"), id(cust), nil, s("M"), s("751234567V"), s("Penicillin"), s(""), &ts}}},
		{Name: "pharmacy_prescriptions", Columns: []string{"id", "patient_id", "prescriber_name", "prescriber_reg", "prescribed_on", "notes", "is_repeat", "repeat_interval_days", "created_by", "created_at", "active"},
			Rows: [][]*string{{s("1"), s("1"), s("Dr Silva"), s("SLMC 123"), &day, s(""), s("true"), s("30"), nil, &ts, s("true")}}},
		{Name: "pharmacy_prescription_items", Columns: []string{"id", "prescription_id", "product_id", "drug_text", "dosage", "quantity", "per_day"},
			Rows: [][]*string{{s("1"), s("1"), id(prod), s("Panadol 500mg"), s("1-0-1"), s("60"), s("2")}}},
		{Name: "pharmacy_dispenses", Columns: []string{"id", "patient_id", "prescription_id", "sale_id", "sale_receipt", "dispensed_by", "dispensed_at", "notes", "days_supply"},
			Rows: [][]*string{{s("1"), s("1"), s("1"), nil, s("S-00001"), nil, &ts, s(""), s("15")}}},
		{Name: "pharmacy_dispense_items", Columns: []string{"id", "dispense_id", "product_id", "drug_text", "quantity", "prescription_item_id", "days_supply"},
			Rows: [][]*string{{s("1"), s("1"), id(prod), s("Panadol 500mg"), s("30"), s("1"), s("15")}}},
		{Name: "pharmacy_controlled_products", Columns: []string{"product_id", "schedule", "created_at"},
			Rows: [][]*string{{id(prod), s("Schedule II"), &ts}}},
		{Name: "pharmacy_cd_ledger", Columns: []string{"id", "product_id", "direction", "quantity", "dispense_id", "patient_name", "prescriber_name", "reference", "notes", "entry_at", "created_by"},
			Rows: [][]*string{{s("1"), id(prod), s("in"), s("100"), nil, s(""), s(""), s("INV-9"), s(""), &ts, nil}, {s("2"), id(prod), s("out"), s("30"), s("1"), s("Nimal Perera"), s("Dr Silva"), s("S-00001"), s(""), &ts, nil}}},
	}}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	json.NewEncoder(gz).Encode(arc) //nolint:errcheck
	gz.Close()
	rep, err := backup.Restore(context.Background(), db, &buf)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(rep.DroppedColumns) > 0 || len(rep.SkippedTables) > 0 {
		t.Fatalf("restore left things out: %+v", rep)
	}
	for _, tb := range dataTables[:7] {
		var n int
		must(db.Get(&n, `SELECT COUNT(*) FROM `+tb))
		if n == 0 {
			t.Fatalf("%s came back empty", tb)
		}
	}
	// New rows after a restore get fresh ids.
	var n int64
	must(db.Get(&n, `INSERT INTO pharmacy_prescriptions (patient_id) VALUES (1) RETURNING id`))
	if n <= 1 {
		t.Fatalf("new prescription id %d", n)
	}
}
