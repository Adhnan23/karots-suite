package gen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/datacheck"
)

// openShopDB makes an empty, migrated database of its own on the test server
// (DATABASE_URL), with the shop's server wired; drop removes it.
func openShopDB() (d Deps, drop func(), err error) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		return d, nil, errNoDB
	}
	admin, err := db.Connect(base)
	if err != nil {
		return d, nil, err
	}
	name := fmt.Sprintf("demo_gen_%d", rand.Uint32())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		return d, nil, err
	}
	admin.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`) // as the demo runner does
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	sh, err := db.Connect(base[:i+1] + name + q)
	if err != nil {
		return d, nil, err
	}
	drop = func() {
		sh.Close()
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		admin.Close()
	}
	if err := app.Migrate(sh); err == nil {
		err = app.EnsureSystemAdmin(sh, "")
	}
	if err != nil {
		drop()
		return d, nil, err
	}
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	return Deps{DB: sh, Svc: app.NewServices(sh, cfg, v), API: app.BuildServer(sh, cfg, v),
		ReceiptPrinter: "127.0.0.1:9101", LabelPrinter: "127.0.0.1:9102"}, drop, nil
}

var errNoDB = errors.New("DATABASE_URL not set")

func newShopDB(t *testing.T) Deps {
	t.Helper()
	d, drop, err := openShopDB()
	if errors.Is(err, errNoDB) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(drop)
	return d
}

var nop = func(int, string) {}

// freshShop builds profile p live: days of past, then today.
func freshShop(t *testing.T, p shops.Profile, now time.Time, days int) *sqlx.DB {
	t.Helper()
	d := newShopDB(t)
	if err := Run(context.Background(), d, p, now, days, nop); err != nil {
		t.Fatal(err)
	}
	return d.DB
}

var (
	groceryOnce sync.Once
	groceryDB   *sqlx.DB
	groceryTook time.Duration // restore + today: what a customer waits for
	groceryNow  time.Time
	groceryErr  error
)

// grocery goes the launch way: its year is built as of three days ago and kept
// as a backup (as `make demo` does), then restored into a fresh database and
// brought up to today. Built once; TestMain drops it.
func grocery(t *testing.T) (*sqlx.DB, time.Time) {
	groceryOnce.Do(func() { groceryErr = buildGrocery() })
	if errors.Is(groceryErr, errNoDB) {
		t.Skip(groceryErr)
	}
	if groceryErr != nil {
		t.Fatal(groceryErr)
	}
	return groceryDB, groceryNow
}

func buildGrocery() error {
	groceryNow = time.Now()
	p, _ := shops.ByKey("grocery")
	p.Volume = 25 // a test-sized year; the real one is built by make demo
	ctx := context.Background()
	past, dropPast, err := openShopDB()
	if err != nil {
		return err
	}
	defer dropPast()
	if err := Past(ctx, past, p, groceryNow.AddDate(0, 0, -3), historyDays, nop); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := backup.Dump(ctx, past.DB, groceryNow.Format(time.RFC3339), &buf, backup.DumpOpts{}); err != nil {
		return err
	}
	d, drop, err := openShopDB()
	if err != nil {
		return err
	}
	dropAfter = append(dropAfter, drop)
	began := time.Now()
	if _, err := backup.Restore(ctx, d.DB, &buf); err != nil {
		return err
	}
	if err := Today(ctx, d, p, groceryNow, nop); err != nil {
		return err
	}
	groceryTook, groceryDB = time.Since(began), d.DB
	return nil
}

var dropAfter []func()

func TestMain(m *testing.M) {
	code := m.Run()
	for _, f := range dropAfter {
		f()
	}
	os.Exit(code)
}

func count(t *testing.T, d *sqlx.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := d.Get(&n, q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestYearOfHistory(t *testing.T) {
	d, now := grocery(t)
	day := func(ago int) time.Time { return localDay(now).AddDate(0, 0, -ago) }
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at >= $1`, day(0)); n == 0 {
		t.Error("no sales today")
	}
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at >= $1 AND created_at < $2`, day(7), day(6)); n == 0 {
		t.Error("no sales a week ago")
	}
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at < $1`, day(365)); n == 0 {
		t.Error("no sales a year ago")
	}
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at > $1`, now); n != 0 {
		t.Errorf("%d sales in the future", n)
	}
	if n := count(t, d, `SELECT count(*) FROM purchases WHERE created_at > $1`, now); n != 0 {
		t.Errorf("%d purchases in the future", n)
	}
}

func TestTodayHasOpenTill(t *testing.T) {
	d, _ := grocery(t)
	if n := count(t, d, `SELECT count(*) FROM cash_register WHERE closed_at IS NULL`); n < 1 {
		t.Error("no till open today")
	}
}

func TestIntegrity(t *testing.T) {
	d, _ := grocery(t)
	f, err := datacheck.Run(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("data check findings: %+v", f)
	}
}

func TestLowStockAndAutoLevels(t *testing.T) {
	d, _ := grocery(t)
	if n := count(t, d, `SELECT count(*) FROM products WHERE reorder_level > 0`); n < 10 {
		t.Errorf("only %d products have a learned reorder level", n)
	}
}

func TestSettingsPrintImageToEmulators(t *testing.T) {
	d, _ := grocery(t)
	var s struct {
		PrintAs string `db:"print_as"`
		Label   string `db:"label_print_as"`
		RP      string `db:"receipt_printer"`
		LP      string `db:"label_printer"`
		Skin    string `db:"skin"`
	}
	if err := d.Get(&s, `SELECT print_as, label_print_as, receipt_printer, label_printer, skin FROM settings WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if s.PrintAs != "image" || s.Label != "image" || s.RP != "tcp://127.0.0.1:9101" || s.LP != "tcp://127.0.0.1:9102" || s.Skin != "studio" {
		t.Fatalf("settings = %+v", s)
	}
}

func TestFast(t *testing.T) {
	grocery(t)
	t.Logf("grocery restored and brought to today in %s", groceryTook)
	if groceryTook > 30*time.Second {
		t.Errorf("opening the grocery took %s, want ≤ 30 s", groceryTook)
	}
}

func small(key string) shops.Profile {
	p, _ := shops.ByKey(key)
	p.Volume = 6
	return p
}

func TestJustAfterMidnight(t *testing.T) {
	now := localDay(time.Now()).Add(5 * time.Minute)
	if now.Weekday() != time.Sunday { // whatever today is, the rule is the same
		t.Logf("today is %s", now.Weekday())
	}
	d := freshShop(t, small("grocery"), now, 30)
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at >= $1`, localDay(now)); n == 0 {
		t.Error("no sales just after midnight")
	}
	if n := count(t, d, `SELECT count(*) FROM sales WHERE created_at > $1`, now); n != 0 {
		t.Errorf("%d sales after now", n)
	}
	if n := count(t, d, `SELECT count(*) FROM cash_register WHERE closed_at IS NULL`); n < 1 {
		t.Error("no till open just after midnight")
	}
}

func TestDeterministic(t *testing.T) {
	now := time.Now()
	sum := func() string {
		d := freshShop(t, small("grocery"), now, 30)
		var s string
		if err := d.Get(&s, `SELECT count(*)::text || ' ' || COALESCE(sum(total), 0)::text FROM sales`); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if a, b := sum(), sum(); a != b {
		t.Fatalf("two runs differ: %s vs %s", a, b)
	}
}

func TestBackOffice(t *testing.T) {
	d, now := grocery(t)
	monthAgo := localDay(now).AddDate(0, -2, 0)
	for what, q := range map[string]string{
		"supplier payments":      `SELECT count(*) FROM supplier_payments`,
		"customer payments":      `SELECT count(*) FROM customer_payments`,
		"expenses in two months": `SELECT count(*) FROM expenses WHERE expense_date >= '` + monthAgo.Format("2006-01-02") + `'`,
		"returns":                `SELECT count(*) FROM sale_returns`,
		"damage write-offs":      `SELECT count(*) FROM stock_movements WHERE type = 'damage'`,
		"products below reorder": `SELECT count(*) FROM products p WHERE p.reorder_level > 0 AND (SELECT COALESCE(sum(qty_remaining), 0) FROM stock_batches b WHERE b.product_id = p.id) < p.reorder_level`,
	} {
		if n := count(t, d, q); n == 0 {
			t.Errorf("no %s", what)
		}
	}
}

func TestCreditWithinLimitsAndSafeIsDefault(t *testing.T) {
	d, _ := grocery(t)
	if n := count(t, d, `SELECT count(*) FROM customers WHERE outstanding_balance > credit_limit`); n != 0 {
		t.Errorf("%d customers over their credit limit (only the cheat puts one there)", n)
	}
	if n := count(t, d, `SELECT count(*) FROM settings WHERE default_locker_id IS NOT NULL`); n != 1 {
		t.Error("no default safe: the setup checklist would still nag a year-old shop")
	}
}

// Opened on a Monday before the 07:30 delivery: nothing may be stamped after now.
func TestNothingAfterNowOnMondayMorning(t *testing.T) {
	now := localDay(time.Now())
	for now.Weekday() != time.Monday || !now.Before(time.Now()) {
		now = now.AddDate(0, 0, -1)
	}
	now = now.Add(6 * time.Hour)
	d := freshShop(t, small("grocery"), now, 30)
	tm, err := newTimeMachine(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	trade := map[string]bool{"sales": true, "payments": true, "purchases": true, "stock_batches": true, "stock_movements": true, "cash_register": true, "expenses": true}
	for _, c := range tm.tsCols {
		if c.column != "created_at" || !trade[c.table] { // sign-ins are at the real clock; this "now" is in the past
			continue
		}
		if n := count(t, d, fmt.Sprintf(`SELECT count(*) FROM %q WHERE created_at > $1`, c.table), now.Add(time.Minute)); n != 0 {
			t.Errorf("%s: %d rows after now", c.table, n)
		}
	}
}
