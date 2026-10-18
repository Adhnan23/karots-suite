package recharge

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

// A karots-pos recharge backup (its 00019 columns exactly) restores with no
// mapping: the device reads the float karots-pos showed (last close + every move
// since), the old service charges and closed-session bonus stay earnings, the
// float it bought as an expense is added back, and a suite count adds on top.
func TestKarotsPosRechargeRestore(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	var product, expense int64
	must(t, db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Recharge') RETURNING id`))
	must(t, db.Get(&product, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service, pass_through)
		VALUES ('Dialog Recharge', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 0, true, true) RETURNING id`))
	at := func(h int) *string {
		v := time.Now().Add(time.Duration(h-100) * time.Hour).UTC().Format(time.RFC3339)
		return &v
	}
	must(t, db.Get(&expense, `INSERT INTO expenses (category, amount, description, expense_date) VALUES ('Float top-up', 2000, 'Dialog supplier float refill', $1) RETURNING id`, *at(4)))
	s := func(v string) *string { return &v }
	id := func(v int64) *string { return s(strconv.FormatInt(v, 10)) }
	txCols := []string{"id", "session_id", "carrier_id", "device_id", "type", "amount", "cash_delta", "float_delta", "sale_id", "expense_id",
		"reference", "note", "created_by", "created_at", "service_charge", "reversal_of", "reversed_at", "cash_given"}
	row := func(id, sess, typ, amt, cash, flt, svc string, exp *string, h int) []*string {
		return []*string{s(id), s(sess), s("1"), s("1"), s(typ), s(amt), s(cash), s(flt), nil, exp, nil, nil, nil, at(h), s(svc), nil, nil, nil}
	}
	arc := backup.Archive{Version: backup.FormatVersion, Partial: true, Groups: []string{"recharge"}, Tables: []backup.Table{
		{Name: "recharge_carriers", Columns: []string{"id", "name", "product_id", "is_active", "created_at"},
			Rows: [][]*string{{s("1"), s("Dialog"), id(product), s("true"), at(0)}}},
		{Name: "recharge_devices", Columns: []string{"id", "carrier_id", "label", "number", "is_active", "for_recharge", "for_money", "tracks_float", "recommended_float"},
			Rows: [][]*string{{s("1"), s("1"), s("Shop phone"), s("0771234567"), s("true"), s("true"), s("true"), s("true"), s("8000")}}},
		{Name: "recharge_device_sessions", Columns: []string{"id", "session_id", "device_id", "opening", "closing", "opened_at", "closed_at"},
			Rows: [][]*string{
				{s("1"), s("10"), s("1"), s("5000"), s("4150"), at(1), at(3)},
				{s("2"), s("11"), s("1"), s("6150"), nil, at(5), nil}, // still open: karots-pos ignored its opening too
			}},
		{Name: "recharge_transactions", Columns: txCols, Rows: [][]*string{
			row("1", "0", "opening", "5000", "0", "5000", "0", nil, 0),
			row("2", "10", "reload", "1000", "0", "-1000", "0", nil, 2),
			row("3", "10", "deposit", "300", "300", "-300", "20", nil, 2),
			row("4", "0", "refill", "2000", "0", "2000", "0", id(expense), 4),
			row("5", "11", "reload", "500", "0", "-500", "0", nil, 6),
		}},
		{Name: "recharge_bill_tx", Columns: []string{"id", "session_id", "bank_locker_id", "bank_name", "type", "amount", "service_charge",
			"reference", "note", "created_by", "created_at", "cash_given", "device_id", "customer_id", "bill_type"},
			Rows: [][]*string{{s("1"), s("10"), nil, s("Dialog · Shop phone"), s("billpay"), s("2500"), s("50"), s("0123456"), nil, nil, at(2), nil, s("1"), nil, s("Electricity")}}},
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
	fs, err := st.Floats(ctx, db, false)
	must(t, err)
	// karots-pos DeviceBalances: last close 4150 + refill 2000 − reload 500.
	if len(fs) != 1 || !fs[0].Balance.Equal(d("5650")) || !fs[0].Target.Equal(d("8000")) {
		t.Fatalf("restored float: %+v", fs)
	}
	all := [2]time.Time{time.Now().Add(-200 * time.Hour), time.Now().Add(time.Hour)}
	// Bonus at close 4150 − (5000 − 1000 − 300) = 450, + service charges 20 + 50.
	if e, err := Earnings(ctx, db, all[0], all[1]); err != nil || !e.Equal(d("520")) {
		t.Fatalf("old earnings %s %v, want 520", e, err)
	}
	if b, err := OldFloatBought(ctx, db, all[0], all[1]); err != nil || !b.Equal(d("2000")) {
		t.Fatalf("old float bought %s %v, want 2000", b, err)
	}
	if owner, _ := st.expenseOwner(ctx, expense); owner == "" {
		t.Fatal("the old float top-up expense can be voided")
	}

	res, err := st.Count(ctx, []CountInput{{DeviceID: 1, Counted: d("5700")}}, nil, 0)
	must(t, err)
	if len(res) != 1 || !res[0].Diff.Equal(d("50")) {
		t.Fatalf("count: %+v", res)
	}
	if b, _ := balance(ctx, db, 1); !b.Equal(d("5700")) {
		t.Fatalf("after the count the float is %s, want 5700", b)
	}
	if e, _ := Earnings(ctx, db, all[0], all[1]); !e.Equal(d("570")) {
		t.Fatalf("earnings after the count %s, want 570", e)
	}
}
