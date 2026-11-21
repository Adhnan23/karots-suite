package ltest

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"
)

// FaultyShop is a Shop on a database that can be made to fail (PLG-4), plus a
// clean connection to look at it with.
type FaultyShop struct {
	*Shop
	Fault *Fault
	Look  *sqlx.DB
}

// Plugin hooks are process-wide (each BuildServer runs every plugin's Setup
// again, and the last database wins), so a plugin's test binary holds ONE shop:
// Main opens it before the tests and drops it after; Shared hands it out. Tests
// make their own rows and look them up exactly; tests that compare every table
// (Sweep, refusals) don't call t.Parallel, so they run alone, before the rest.
var shared *FaultyShop

// Main runs a plugin package's tests around its one shop:
//
//	func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }
func Main(m *testing.M) int {
	flag.Parse()
	mt := &mainT{}
	defer mt.cleanup()
	if !testing.Short() && os.Getenv("DATABASE_URL") != "" {
		func() {
			defer func() {
				if r := recover(); r != nil && r != errMainSkip {
					panic(r)
				}
			}()
			shared = newFaultyShop(mt)
		}()
	}
	return m.Run()
}

// Shared is the package's one shop; skipped when Main didn't open one.
func Shared(t T) *FaultyShop {
	helper(t)
	if shared == nil {
		t.Skip("battle: no shop (-short, no DATABASE_URL, or no ltest.Main)")
	}
	return shared
}

func newFaultyShop(t T) *FaultyShop {
	db, dsn, f := NewFaultyDB(t)
	look, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { look.Close() })
	return &FaultyShop{Shop: NewShopOn(t, db), Fault: f, Look: look}
}

var errMainSkip = fmt.Errorf("ltest: skipped")

// mainT is the T a TestMain has: cleanups run when the tests end.
type mainT struct{ fns []func() }

func (m *mainT) Fatal(args ...any)                 { panic(fmt.Sprint(args...)) }
func (m *mainT) Fatalf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }
func (m *mainT) Errorf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }
func (m *mainT) Skip(args ...any)                  { panic(errMainSkip) }
func (m *mainT) Cleanup(f func())                  { m.fns = append(m.fns, f) }
func (m *mainT) cleanup() {
	for i := len(m.fns) - 1; i >= 0; i-- {
		m.fns[i]()
	}
}

// State is a hash of every table's rows but the audit log and goose's, looked
// at through q.
func State(t T, q sqlx.Queryer) map[string]string {
	helper(t)
	var tables []string
	if err := sqlx.Select(q, &tables, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY 1`); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, tb := range tables {
		if tb == "audit_log" || strings.HasPrefix(tb, "goose_db_version") {
			continue
		}
		var h *string
		if err := sqlx.Get(q, &h, fmt.Sprintf(`SELECT md5(string_agg(x::text, '|' ORDER BY x::text)) FROM %q x`, tb)); err != nil {
			t.Fatal(err)
		}
		out[tb] = ""
		if h != nil {
			out[tb] = *h
		}
	}
	return out
}

// Changed lists the tables whose rows differ between two States.
func Changed(before, after map[string]string) []string {
	var out []string
	for tb, h := range after {
		if before[tb] != h {
			out = append(out, tb)
		}
	}
	sort.Strings(out)
	return out
}

// MoneyAndStock are the tables a product-data plugin never changes (PLG-1).
var MoneyAndStock = []string{"sales", "sale_items", "sale_payments", "sale_returns", "sale_return_items",
	"stock", "stock_batches", "stock_movements", "cash_movements", "till_sessions", "lockers",
	"customers", "customer_payments", "suppliers", "supplier_payments", "purchases", "purchase_items", "expenses"}

// Untouched fails when any of tables changed between two States (a missing
// table is skipped: not every name is in every schema).
func Untouched(t T, before, after map[string]string, tables []string) {
	helper(t)
	for _, tb := range Changed(before, after) {
		if slices.Contains(tables, tb) {
			t.Errorf("PLG-1: %s changed", tb)
		}
	}
}

// Sweep runs act with a failure planted at statement 1, 2, … until it runs
// clean; after each failure every table is as before and the auditor passes
// (PLG-4). setup (may be nil) runs before each try, unfailed.
func (s *FaultyShop) Sweep(t *testing.T, name string, setup func(), act func() error) {
	t.Helper()
	for k := 1; k < 400; k++ {
		if setup != nil {
			setup()
		}
		before := State(t, s.Look)
		s.Fault.Arm(k)
		err := act()
		fired := s.Fault.Fired()
		failed := s.Fault.Failed()
		if !fired {
			if err != nil {
				t.Fatalf("%s: failed with nothing planted (after %d tries): %v", name, k, err)
			}
			t.Logf("%s: %d statements, each failure left nothing behind", name, k-1)
			return
		}
		if err == nil {
			t.Logf("%s: statement %d failed and the action still succeeded: %.90s", name, k, failed)
		} else {
			for _, tb := range Changed(before, State(t, s.Look)) {
				t.Errorf("PLG-4 %s: failing statement %d (%.90s) still changed %s", name, k, failed, tb)
			}
		}
		Check(t, t.Context(), s.Look)
	}
	t.Fatalf("%s: still failing after 400 statements", name)
}

var tableRe = regexp.MustCompile(`(?i)(CREATE|DROP) TABLE (?:IF (?:NOT )?EXISTS )?(\w+)`)

// DataGroupHoldsTables checks PLG-7: every table key's migrations leave behind
// is listed in one of its data groups. Call it after a Shop is open (data groups
// are registered by the plugins' Setup).
func DataGroupHoldsTables(t T, key string) {
	helper(t)
	var mig fs.FS
	for _, p := range plugin.All() {
		if p.Key() == key {
			mig, _ = p.Migrations()
		}
	}
	if mig == nil {
		t.Fatalf("plugin %s has no migrations here", key)
	}
	files, _ := fs.Glob(mig, "*.sql")
	sort.Strings(files)
	made := map[string]bool{}
	for _, f := range files {
		b, _ := fs.ReadFile(mig, f)
		up, _, _ := strings.Cut(string(b), "+goose Down")
		for _, m := range tableRe.FindAllStringSubmatch(up, -1) {
			made[m[2]] = strings.EqualFold(m[1], "CREATE")
		}
	}
	listed := map[string]bool{}
	for _, g := range plugin.CollectDataGroups() {
		for _, tb := range g.Tables {
			listed[tb] = true
		}
	}
	for tb, kept := range made {
		if kept && !listed[tb] {
			t.Errorf("PLG-7: %s makes table %s but no data group lists it", key, tb)
		}
	}
}
