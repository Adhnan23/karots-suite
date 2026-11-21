package battle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/ledgercheck/ltest"
)

// A database failure part-way through any money or stock action leaves nothing
// half written: the sweep fails its 1st statement, then its 2nd, … until it runs
// clean, and after every failure each table is exactly as before and the auditor
// passes. This reaches the `if err != nil` branches no other test can.

// sweepShop is a shop on a database that can be made to fail, plus a clean
// connection to look at it with.
type sweepShop struct {
	*shop
	fault *ltest.Fault
	look  *sqlx.DB
}

func newSweepShop(t *testing.T) *sweepShop {
	if testing.Short() {
		t.Skip("battle: skipped with -short")
	}
	db, dsn, f := ltest.NewFaultyDB(t)
	look, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { look.Close() })
	return &sweepShop{shop: newShopOn(t, db, dsn), fault: f, look: look}
}

// noise is what may change on a failed request: who's signed in where, and the
// background bookkeeping no rule reads.
var noise = map[string]bool{"audit_log": true}

// state is a hash of every table's rows.
func (s *sweepShop) state(t *testing.T) map[string]string {
	t.Helper()
	var tables []string
	s.must(t, s.look.Select(&tables, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY 1`))
	out := map[string]string{}
	for _, tb := range tables {
		if noise[tb] || strings.HasPrefix(tb, "goose_db_version") {
			continue
		}
		var h *string
		s.must(t, s.look.Get(&h, fmt.Sprintf(`SELECT md5(string_agg(x::text, '|' ORDER BY x::text)) FROM %q x`, tb)))
		if h != nil {
			out[tb] = *h
		} else {
			out[tb] = ""
		}
	}
	return out
}

// sweep runs act with a failure planted at statement 1, 2, … until it runs
// clean. setup (may be nil) runs before each try, unfailed.
func (s *sweepShop) sweep(t *testing.T, name string, setup func(), act func() error) {
	t.Helper()
	for k := 1; k < 400; k++ {
		if setup != nil {
			setup()
		}
		before := s.state(t)
		s.fault.Arm(k)
		err := act()
		fired := s.fault.Fired()
		failed := s.fault.Failed()
		if !fired {
			if err != nil {
				t.Fatalf("%s: failed with nothing planted (after %d tries): %v", name, k, err)
			}
			t.Logf("%s: %d statements, each failure left nothing behind", name, k-1)
			return
		}
		if err == nil {
			// The action finished although a statement failed: only fine for
			// best-effort extras; the auditor still has to pass.
			t.Logf("%s: statement %d failed and the action still succeeded: %.90s", name, k, failed)
		} else {
			after := s.state(t)
			for tb, h := range after {
				if before[tb] != h {
					t.Errorf("%s: failing statement %d (%.90s) still changed %s", name, k, failed, tb)
				}
			}
		}
		s.audit(t)
	}
	t.Fatalf("%s: still failing after 400 statements", name)
}

func TestSweepSale(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	s.open(t, s.till1)
	cust := s.newCustomer(t, "0712000901", "100000", "")
	s.sweep(t, "cash sale", nil, func() error {
		return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "5", "discountType": "fixed",
			"items":    []line{{"productId": s.prod["plain"], "quantity": "2"}, {"productId": s.prod["pack"], "quantity": "13"}},
			"payments": []pay{cash("500")}}, nil)
	})
	s.sweep(t, "sale on account", nil, func() error {
		_, err := s.onAccount(t, cust, 3, "100", nil)
		return err
	})
}

func TestSweepReturn(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "5"}}, cash("500"))
	item := s.sale(t, s.till1, r.ID).Lines[0].ID
	s.sweep(t, "return at the counter", nil, func() error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
			"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, nil)
	})
}

// DRAW-18: a withdraw that was saved is answered as done, whatever fails after.
func TestSweepDrawer(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	safe := s.locker(t, "Safe", "safe", "0")
	// A failed receipt label after the open still opens the till: close it so
	// every try starts from a closed till.
	reclose := func() {
		_ = s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{"1000": 5}, "destLockerId": safe}, nil)
	}
	s.sweep(t, "open the till", reclose, func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}, nil)
	})
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	s.sweep(t, "withdraw to the safe", nil, func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "1000", "reason": "safe", "lockerId": safe}, nil)
	})
	// Likewise a close that went through despite a failed extra: open it again
	// (409 while it's still open — fine) so every try closes an open till.
	reopen := func() {
		_ = s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"100": 41}}, nil)
	}
	s.sweep(t, "close into the safe", reopen, func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{"100": 41}, "destLockerId": safe}, nil)
	})
}

func TestSweepBuying(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	safe := s.locker(t, "Safe", "safe", "100000")
	sup := s.newSup(t, "Sweep Ltd", "500")
	src := fmt.Sprintf("locker:%d", safe)
	s.sweep(t, "delivery paid in part", nil, func() error {
		_, err := s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "40"},
			{"productId": s.prod["odd"], "quantity": "3", "costPrice": "3.33"}}, map[string]any{"paidAmount": "100", "source": src})
		return err
	})
	s.sweep(t, "supplier payment", nil, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]string{"amount": "300", "method": "cash", "source": src}, nil)
	})
}

func TestSweepStock(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	p := s.stocked(t, "Sweep bulbs", s.unitPcs, 0, nil)
	s.addLot(t, p, "20", "10", "2027-01-31")
	s.addLot(t, p, "20", "12", "")
	s.sweep(t, "write-off", nil, func() error {
		return s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "3", "reason": "damage"}, nil)
	})
	s.sweep(t, "count", nil, func() error {
		return s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "30", "note": "count"}, nil)
	})
	box := s.stocked(t, "Sweep box", s.unitPcs, 0, nil)
	s.addLot(t, box, "2", "100", "")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": box, "toProductId": p, "ratio": "10"}, &cv))
	s.sweep(t, "conversion run", nil, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "1"}, nil)
	})
}

func TestSweepCustomerPayment(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	s.open(t, s.till1)
	cust := s.newCustomer(t, "0712000902", "100000", "1000")
	s.sweep(t, "customer pays at the till", nil, func() error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cust), map[string]any{"amount": "250", "method": "cash"}, nil)
	})
}
