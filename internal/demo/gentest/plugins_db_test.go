// Package gentest builds plugin shops. Plugins register once per process, so a
// run builds one shop: DEMO_SHOP picks it (default: everything, every plugin).
//
//	DEMO_SHOP=phone go test ./internal/demo/gentest
package gentest

import (
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	_ "karots-suite/internal/demo/allplugins"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/datacheck"
	"karots-suite/internal/plugin"
)

var (
	shop    shops.Profile
	shopDB  *sqlx.DB
	shopErr error
	now     = time.Now()
	calls   = map[string]int{} // plugin API calls made, by plugin key
)

func TestMain(m *testing.M) {
	key := os.Getenv("DEMO_SHOP")
	if key == "" {
		key = "everything"
	}
	var ok bool
	if shop, ok = shops.ByKey(key); !ok {
		fmt.Println("no such shop:", key)
		os.Exit(2)
	}
	keys := shop.Plugins
	if keys == nil {
		keys = []string{}
	}
	plugin.Only(keys)
	shop.Volume = 15 // a test-sized shop
	drop, err := build()
	shopErr = err
	code := m.Run()
	if drop != nil {
		drop()
	}
	os.Exit(code)
}

// counting records which plugins' routes the generator called.
func counting(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rest, ok := strings.CutPrefix(r.URL.Path, "/api/v1/x/"); ok {
			k, _, _ := strings.Cut(rest, "/")
			calls[k]++
		}
		h.ServeHTTP(w, r)
	})
}

// openDB is an empty migrated database on the test server, and its server.
func openDB(base string, admin *sqlx.DB) (gen.Deps, string, error) {
	name := fmt.Sprintf("demo_gentest_%d", rand.Uint32())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		return gen.Deps{}, "", err
	}
	admin.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`)
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	sh, err := db.Connect(base[:i+1] + name + q)
	if err != nil {
		return gen.Deps{}, name, err
	}
	if err := app.Migrate(sh); err != nil {
		return gen.Deps{}, name, err
	}
	if err := app.EnsureSystemAdmin(sh, ""); err != nil {
		return gen.Deps{}, name, err
	}
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	return gen.Deps{DB: sh, Svc: app.NewServices(sh, cfg, v), API: counting(app.BuildServer(sh, cfg, v)),
		ReceiptPrinter: "127.0.0.1:9101", LabelPrinter: "127.0.0.1:9102"}, name, nil
}

// build goes the launch way: the past as of three days ago, kept as a backup
// (as make demo does), restored into a fresh database and brought to today.
func build() (func(), error) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		return nil, nil
	}
	admin, err := db.Connect(base)
	if err != nil {
		return nil, err
	}
	var names []string
	var dbs []*sqlx.DB
	drop := func() {
		for _, d := range dbs {
			d.Close()
		}
		for _, n := range names {
			admin.Exec(`DROP DATABASE IF EXISTS ` + n + ` WITH (FORCE)`)
		}
		admin.Close()
	}
	ctx := context.Background()
	past, name, err := openDB(base, admin)
	names = append(names, name)
	if err != nil {
		return drop, err
	}
	dbs = append(dbs, past.DB)
	if err := gen.Past(ctx, past, shop, now.AddDate(0, 0, -3), 60, func(int, string) {}); err != nil {
		return drop, err
	}
	var buf bytes.Buffer
	if err := backup.Dump(ctx, past.DB, now.Format(time.RFC3339), &buf, backup.DumpOpts{}); err != nil {
		return drop, err
	}
	d, name, err := openDB(base, admin)
	names = append(names, name)
	if err != nil {
		return drop, err
	}
	dbs = append(dbs, d.DB)
	shopDB = d.DB
	if _, err := backup.Restore(ctx, d.DB, &buf); err != nil {
		return drop, err
	}
	return drop, gen.Today(ctx, d, shop, now, func(int, string) {})
}

func need(t *testing.T) *sqlx.DB {
	t.Helper()
	if shopDB == nil && shopErr == nil {
		t.Skip("DATABASE_URL not set")
	}
	if shopErr != nil {
		t.Fatal(shopErr)
	}
	return shopDB
}

func count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := shopDB.Get(&n, q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestTodayTraded(t *testing.T) {
	need(t)
	y, m, d := now.Date()
	if n := count(t, `SELECT count(*) FROM sales WHERE created_at >= $1`, time.Date(y, m, d, 0, 0, 0, 0, now.Location())); n == 0 {
		t.Error("no sales today after the restore")
	}
	if n := count(t, `SELECT count(*) FROM sales WHERE created_at > $1`, now); n != 0 {
		t.Errorf("%d sales after now", n)
	}
}

func TestIntegrity(t *testing.T) {
	d := need(t)
	f, err := datacheck.Run(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("data check findings: %+v", f)
	}
}

func TestOnlyActivePluginsCalled(t *testing.T) {
	need(t)
	for k := range calls {
		if !shop.Has(k) {
			t.Errorf("called %s, which %s doesn't run", k, shop.Key)
		}
	}
}

// pluginTables: each plugin's main table and the column that dates it.
var pluginTables = map[string][2]string{
	"repairs":   {"repair_jobs", "created_at"},
	"recharge":  {"recharge_transactions", "created_at"},
	"documents": {"doc_job", "created_at"},
	"combos":    {"combo_sales", "created_at"},
	"pharmacy":  {"pharmacy_dispenses", "dispensed_at"},
	"requests":  {"requests", "created_at"},
}

// setupTables: plugins set up once, with no daily life.
var setupTables = map[string]string{"alternatives": "alt_members", "productplus": "pp_values", "clearance": "clearance_settings"}

func TestEachPluginIsSetUp(t *testing.T) {
	need(t)
	for _, k := range shop.Plugins {
		if tb, ok := setupTables[k]; ok && count(t, `SELECT count(*) FROM `+tb) == 0 {
			t.Errorf("%s: nothing in %s", k, tb)
		}
	}
}

func TestEachPluginHasAPast(t *testing.T) {
	d := need(t)
	day := func(ago int) time.Time {
		y, m, dd := now.AddDate(0, 0, -ago).Date()
		return time.Date(y, m, dd, 0, 0, 0, 0, now.Location())
	}
	for _, k := range shop.Plugins {
		tc, ok := pluginTables[k]
		if !ok {
			continue
		}
		total := count(t, fmt.Sprintf(`SELECT count(*) FROM %s`, tc[0]))
		old := count(t, fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s < $1`, tc[0], tc[1]), day(45))
		if total < 5 || old == 0 {
			t.Errorf("%s: %d rows, %d older than 45 days", k, total, old)
		}
		_ = d
	}
}
