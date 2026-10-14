package repairs

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
	"karots-suite/plugins/repairs/migrations"

	"github.com/jmoiron/sqlx"
)

// freshDB is a fresh database with core and repairs migrated.
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
	const name = "repairs_test_db"
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
		appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_repairs"),
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

// A karots-pos repairs backup (its 0001–0003 columns exactly) restores with no
// mapping: the open job keeps its held deposit and frozen part discount, the old
// "Repair Service" stays the labour product, and the ticket counter carries on.
func TestKarotsPosRepairsRestore(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	var cust, part, labour int64
	must(t, db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Phones') RETURNING id`))
	must(t, db.Get(&cust, `INSERT INTO customers (name, phone) VALUES ('Kamal Silva', '0712223334') RETURNING id`))
	must(t, db.Get(&part, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('Galaxy A12 screen', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 2500, 4000) RETURNING id`))
	must(t, db.Get(&labour, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('Repair Service', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 0, true) RETURNING id`))

	ts := time.Now().Add(-3 * 24 * time.Hour).UTC().Format(time.RFC3339)
	day := time.Now().Add(2 * 24 * time.Hour).UTC().Format("2006-01-02")
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"repairs"}, Tables: []backup.Table{
		{Name: "repair_config", Columns: []string{"id", "labour_product_id", "default_warranty_days", "ticket_seq", "warranty_mode"},
			Rows: [][]*string{{s("1"), id(labour), s("30"), s("7"), s("days")}}},
		{Name: "repair_jobs", Columns: []string{"id", "ticket_no", "ticket_code", "customer_id", "customer_name", "customer_phone",
			"repair_type", "device_model", "fault", "status", "promised_date", "urgent", "warranty_days", "warranty_until", "sale_id",
			"rework_of", "notes", "created_by", "created_at", "ready_at", "collected_at", "cancelled_at", "repaired_by", "repairer_paid"},
			Rows: [][]*string{{s("7"), s("R-0007"), s("RPR000007"), id(cust), s("Kamal Silva"), s("0712223334"), s("Screen"),
				s("Galaxy A12"), s("Cracked screen"), s("in_progress"), &day, s("false"), s("30"), nil, nil, nil, s(""), nil, &ts,
				nil, nil, nil, s("Sunil"), s("150")}}},
		{Name: "repair_parts", Columns: []string{"id", "job_id", "product_id", "qty", "unit_charge", "discount", "discount_type", "discount_value", "warranty_days"},
			Rows: [][]*string{{s("1"), s("7"), id(part), s("1"), s("3800"), s("400"), s("percent"), s("10"), s("0")}}},
		{Name: "repair_charges", Columns: []string{"id", "job_id", "label", "amount"},
			Rows: [][]*string{{s("1"), s("7"), s("Labour"), s("1500")}}},
		{Name: "repair_payments", Columns: []string{"id", "job_id", "amount", "kind", "user_id", "created_at"},
			Rows: [][]*string{{s("1"), s("7"), s("1000"), s("deposit"), nil, &ts}}},
	}}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	json.NewEncoder(gz).Encode(arc) //nolint:errcheck
	gz.Close()
	rep, err := backup.Restore(ctx, db, &buf)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(rep.DroppedColumns) > 0 || len(rep.SkippedTables) > 0 {
		t.Fatalf("restore left things out: %+v", rep)
	}
	st := NewStore(db)
	det, err := st.GetJob(ctx, 7)
	must(t, err)
	// Estimate at today's price, the old 10% kept: 4000 − 400 + 1500.
	if !estimate(det).Equal(d("5100")) || !legacyHeld(det).Equal(d("1000")) || !repairerPaid(det).Equal(d("150")) {
		t.Fatalf("old job: est %s held %s paid %s", estimate(det), legacyHeld(det), repairerPaid(det))
	}
	if got, err := st.LabourID(ctx); err != nil || got != labour {
		t.Fatalf("labour %d %v (want the restored %d)", got, err, labour)
	}
	nid, err := st.CreateJob(ctx, JobInput{CustomerID: &cust, CustomerName: "Kamal Silva", DeviceModel: "iPhone 11"})
	must(t, err)
	if nd, _ := st.GetJob(ctx, nid); nd.Job.TicketNo != "R-0008" || nd.Job.TicketCode != "RPR000008" {
		t.Fatalf("ticket after restore: %+v", nd.Job)
	}
}
