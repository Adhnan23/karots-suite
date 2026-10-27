// Package gentest builds plugin shops. Plugins register once per process, so a
// run builds one shop: DEMO_SHOP picks it (default: everything, every plugin).
//
//	DEMO_SHOP=phone go test ./internal/demo/gentest
package gentest

import (
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

func build() (func(), error) {
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		return nil, nil
	}
	admin, err := db.Connect(base)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("demo_gentest_%d", rand.Uint32())
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		return nil, err
	}
	admin.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`)
	drop := func() {
		if shopDB != nil {
			shopDB.Close()
		}
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		admin.Close()
	}
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	if shopDB, err = db.Connect(base[:i+1] + name + q); err != nil {
		return drop, err
	}
	if err := app.Migrate(shopDB); err != nil {
		return drop, err
	}
	if err := app.EnsureSystemAdmin(shopDB, ""); err != nil {
		return drop, err
	}
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	d := gen.Deps{DB: shopDB, Svc: app.NewServices(shopDB, cfg, v), API: counting(app.BuildServer(shopDB, cfg, v)),
		ReceiptPrinter: "127.0.0.1:9101", LabelPrinter: "127.0.0.1:9102"}
	return drop, gen.Run(context.Background(), d, shop, now, 60, func(int, string) {})
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
	"repairs":  {"repair_jobs", "created_at"},
	"recharge": {"recharge_transactions", "created_at"},
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
