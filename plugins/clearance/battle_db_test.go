package clearance

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
)

// Clearance's battle tests (docs/rules/plugins/clearance.md): one throwaway shop
// on the real server with this plugin compiled in (ltest.Main).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

type tillCard struct {
	Badges     []string `json:"badges"`
	Suggestion *struct {
		DiscountType  string `json:"discountType"`
		DiscountValue string `json:"discountValue"`
	} `json:"suggestion"`
}

// till is what the till shows for p: card badges, the apply prompt, the ⓘ rows.
func till(t *testing.T, s *ltest.Shop, p int64) (badge, prompt, info bool) {
	t.Helper()
	var c tillCard
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/products/%d", p), nil, &c))
	var d struct{ Rows []struct{ Label string } }
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/products/%d/detail", p), nil, &d))
	for _, b := range c.Badges {
		badge = badge || strings.HasPrefix(b, "Clearance")
	}
	for _, r := range d.Rows {
		info = info || r.Label == "Clearance"
	}
	return badge, c.Suggestion != nil, info
}

func live(t *testing.T, s *ltest.Shop, p int64, want bool, why string) {
	t.Helper()
	b, pr, i := till(t, s, p)
	if b != want || pr != want || i != want {
		t.Errorf("CLR-1 %s: badge %v, prompt %v, ⓘ %v; want all %v", why, b, pr, i, want)
	}
}

func approve(s *ltest.Shop, p int64, pct string) error {
	return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/approve", p), map[string]string{"percent": pct}, nil)
}

// Every check fires on a row that breaks it.
func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Fire soap", "100", "5", nil)
	dup := s.Product(t, "Fire soap dup", "100", "5", nil)
	for _, c := range []struct{ rule, bad string }{
		{"CLR-9", fmt.Sprintf(`INSERT INTO clearance_markdowns (product_id, discount_value, status) VALUES (%d, 0, 'approved')`, p)},
		{"CLR-9", fmt.Sprintf(`INSERT INTO clearance_markdowns (product_id, discount_value, status) VALUES (%d, 10, 'dismissed')`, p)},
		{"CLR-9", `INSERT INTO clearance_markdowns (product_id, discount_value) VALUES (999999, 10)`},
		{"CLR-9", `UPDATE clearance_settings SET min_margin_percent = -1`},
		{"CLR-8", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d; INSERT INTO clearance_markdowns (product_id, discount_value) VALUES (%[2]d, 10)`, p, dup)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(c.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == c.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", c.rule, c.bad)
		}
	}
}

// CLR-1, CLR-5: the till sees exactly the live markdowns, and they end by themselves.
func TestCLR1_LiveMarkdownsOnly(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Old jam", "200", "3", nil)
	live(t, s, p, false, "before approval")
	s.Must(t, approve(s, p, "25"))
	live(t, s, p, true, "approved with stock")

	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": "0", "note": "gone"}, nil))
	live(t, s, p, false, "stock gone")
	s.Must(t, approve(s, p, "25"))
	live(t, s, p, false, "approved with no stock")

	// A lot that came in after the approval sells at full price.
	q := s.Product(t, "Old tea", "200", "4", nil)
	s.Must(t, approve(s, q, "10"))
	s.DB.MustExec(`UPDATE stock_batches SET created_at = now() - interval '2 hours' WHERE product_id = $1`, q)
	s.DB.MustExec(`UPDATE clearance_markdowns SET updated_at = now() - interval '1 hour' WHERE product_id = $1`, q)
	live(t, s, q, true, "approved an hour ago")
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": q, "newQuantity": "10", "note": "fresh"}, nil))
	if live(t, s, q, false, "a fresh lot came in"); t.Failed() {
		var n int
		s.DB.Get(&n, `SELECT count(*) FROM stock_batches WHERE product_id = $1 AND created_at > now() - interval '1 minute'`, q) //nolint:errcheck
		t.Logf("fresh lots in the last minute: %d", n)
	}

	// CLR-5: snoozed is never live; unsnooze puts it back on the list.
	r := s.Product(t, "Old rice", "200", "4", nil)
	s.Must(t, approve(s, r, "10"))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/snooze", r), nil, nil))
	live(t, s, r, false, "snoozed")
	if st := status(t, s, r); st != "snoozed" {
		t.Errorf("CLR-5: snoozed product listed as %q", st)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/unsnooze", r), nil, nil))
	if st := status(t, s, r); st != "" {
		t.Errorf("CLR-5: brought back, listed as %q, want a plain candidate", st)
	}
	s.Audit(t)
}

// status is a product's row status on the page ("-" = not listed).
func status(t *testing.T, s *ltest.Shop, p int64) string {
	t.Helper()
	var pg PageDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/clearance", nil, &pg))
	for _, r := range pg.Rows {
		if r.ProductID == p {
			return r.Status
		}
	}
	return "-"
}

// CLR-2, CLR-7, PLG-3, PLG-5: bad requests are refused and change nothing.
func TestCLR2_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Refuse soap", "100", "5", nil)
	svc := s.Service(t, "Refuse service", "100")
	off := s.Product(t, "Refuse off", "100", "5", nil)
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off), nil, nil))
	before := ltest.State(t, s.DB)
	for _, c := range []struct {
		name string
		do   func() error
	}{
		{"0 %", func() error { return approve(s, p, "0") }},
		{"100 %", func() error { return approve(s, p, "100") }},
		{"-5 %", func() error { return approve(s, p, "-5") }},
		{"words", func() error { return approve(s, p, "lots") }},
		{"no product", func() error { return approve(s, 999999, "10") }},
		{"a service", func() error { return approve(s, svc, "10") }},
		{"a disabled product", func() error { return approve(s, off, "10") }},
		{"bad id", func() error {
			return s.Admin.Do("POST", "/api/v1/x/clearance/abc/approve", map[string]string{"percent": "10"}, nil)
		}},
		{"snooze no product", func() error { return s.Admin.Do("POST", "/api/v1/x/clearance/999999/snooze", nil, nil) }},
		{"cashier approves", func() error {
			return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/approve", p), map[string]string{"percent": "10"}, nil)
		}},
		{"settings 0 days", func() error { return settings(s, 0, "20", "5") }},
		{"settings 100 %", func() error { return settings(s, 60, "100", "5") }},
		{"settings -1 margin", func() error { return settings(s, 60, "20", "-1") }},
		{"disable none", func() error {
			return s.Admin.Do("POST", "/api/v1/x/clearance/disable", map[string]any{"productIds": []int64{}}, nil)
		}},
	} {
		if err := c.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", c.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", c.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
	// Under the floor is the admin's call (CLR-2).
	s.Must(t, approve(s, p, "95"))
	s.Audit(t)
}

func settings(s *ltest.Shop, stale int, def, margin string) error {
	return s.Admin.Do("PUT", "/api/v1/x/clearance/settings", SettingsDTO{StaleDays: stale, DefaultPercent: def, MinMarginPercent: margin, ExpiryDays: 14, DeadMonths: 6}, nil)
}

// PLG-1: approve, snooze, bring back, settings and disable move no money or stock.
func TestPLG1_NothingMoves(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Still soap", "200", "10", nil)
	dead := s.Product(t, "Still dead", "100", "0", nil)
	s.DB.MustExec(`UPDATE products SET created_at = now() - interval '2 years' WHERE id = $1`, dead)
	before := ltest.State(t, s.DB)
	s.Must(t, approve(s, p, "25"))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/snooze", p), nil, nil))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/unsnooze", p), nil, nil))
	s.Must(t, approve(s, p, "25"))
	s.Must(t, settings(s, 60, "20", "5"))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/clearance/disable", map[string]any{"productIds": []int64{dead}}, nil))
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
}

// CLR-4: at the till the markdown is an ordinary
// line discount and its return refunds what was paid.
func TestCLR4_MarkdownIsALineDiscount(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Sale soap", "200", "10", nil)
	s.Must(t, approve(s, p, "25"))

	var c tillCard
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/products/%d", p), nil, &c))
	if c.Suggestion == nil || c.Suggestion.DiscountType != "percent" || c.Suggestion.DiscountValue != "25" {
		t.Fatalf("CLR-4: suggestion %+v, want 25 percent", c.Suggestion)
	}
	s.OpenTill(t, s.Cashier)
	var r struct {
		ID    int64  `json:"id"`
		Total string `json:"total"`
	}
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items":    []map[string]any{{"productId": p, "quantity": "2", "discount": "25", "discountType": "percent"}},
		"payments": []map[string]any{{"method": "cash", "amount": "300"}}}, &r))
	if r.Total != "300.00" {
		t.Errorf("CLR-4: 2 × 200 at 25 %% off totals %s, want 300.00", r.Total)
	}
	var item int64
	s.Must(t, s.DB.Get(&item, `SELECT id FROM sale_items WHERE sale_id = $1`, r.ID))
	var back struct{ CashRefund string }
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, &back))
	if back.CashRefund != "150.00" {
		t.Errorf("CLR-4 / RET-4: one returned refunds %s, want 150.00 (what was paid)", back.CashRefund)
	}
	// Skipped: the same product sells at full price.
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items":    []map[string]any{{"productId": p, "quantity": "1"}},
		"payments": []map[string]any{{"method": "cash", "amount": "200"}}}, &r))
	if r.Total != "200.00" {
		t.Errorf("CLR-4: skipped markdown totals %s, want 200.00", r.Total)
	}
	s.Audit(t)
}

// CLR-6: Disable retires products still dead, skips any that came back to life.
func TestCLR6_DisableOnlyDead(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	dead := s.Product(t, "Dead lamp", "100", "0", nil)
	alive := s.Product(t, "Dead then stocked", "100", "0", nil)
	fresh := s.Product(t, "New and empty", "100", "0", nil)
	s.DB.MustExec(`UPDATE products SET created_at = now() - interval '2 years' WHERE id = $1`, dead)
	s.DB.MustExec(`UPDATE products SET created_at = now() - interval '2 years' WHERE id = $1`, alive)
	var pg PageDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/clearance", nil, &pg))
	listed := map[int64]bool{}
	for _, d := range pg.Dead {
		listed[d.ProductID] = true
	}
	if !listed[dead] || !listed[alive] || listed[fresh] {
		t.Errorf("CLR-6: dead list %v, want the two old empty ones only", listed)
	}
	// The page is now stale: one got stock since.
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": alive, "newQuantity": "3", "note": "found"}, nil))
	var out struct {
		Disabled int `json:"disabled"`
		Skipped  int `json:"skipped"`
	}
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/clearance/disable", map[string]any{"productIds": []int64{dead, alive, fresh}}, &out))
	if out.Disabled != 1 || out.Skipped != 2 {
		t.Errorf("CLR-6: disabled %d, skipped %d; want 1 and 2", out.Disabled, out.Skipped)
	}
	for id, want := range map[int64]bool{dead: false, alive: true, fresh: true} {
		var active bool
		s.Must(t, s.DB.Get(&active, `SELECT is_active FROM products WHERE id = $1`, id))
		if active != want {
			t.Errorf("CLR-6: product %d active %v, want %v", id, active, want)
		}
	}
	s.Audit(t)
}

// CLR-8, PLG-2: a merge through core keeps the kept product's markdown.
func TestCLR8_Merge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := s.Product(t, "Merge A", "100", "5", nil)
	b := s.Product(t, "Merge B", "100", "5", nil)
	c := s.Product(t, "Merge C", "100", "5", nil)
	s.Must(t, approve(s, a, "10"))
	s.Must(t, approve(s, b, "30"))
	s.Must(t, approve(s, c, "40"))
	merge := func(from, into int64) {
		s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", from, into), map[string]string{"lotPrice": "follow"}, nil))
	}
	merge(a, b)
	var pct string
	s.Must(t, s.DB.Get(&pct, `SELECT discount_value::text FROM clearance_markdowns WHERE product_id = $1`, b))
	if pct != "30" {
		t.Errorf("CLR-8: kept product's markdown %s, want its own 30", pct)
	}
	d := s.Product(t, "Merge D", "100", "5", nil)
	merge(c, d)
	s.Must(t, s.DB.Get(&pct, `SELECT discount_value::text FROM clearance_markdowns WHERE product_id = $1`, d))
	if pct != "40" {
		t.Errorf("CLR-8: kept product without one got %s, want the duplicate's 40", pct)
	}
	s.Audit(t)
}

// PLG-6: approving twice at once ends as one markdown.
func TestPLG6_ApproveAtOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Twice soap", "100", "5", nil)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = approve(s, p, "15") }()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Errorf("PLG-6: %v", err)
		}
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM clearance_markdowns WHERE product_id = $1`, p))
	if n != 1 {
		t.Errorf("PLG-6: %d markdowns, want 1", n)
	}
	s.Audit(t)
}

// PLG-4: a failure at any statement of a write leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	p := s.Product(t, "Sweep soap", "100", "5", nil)
	dead := s.Product(t, "Sweep dead", "100", "0", nil)
	s.DB.MustExec(`UPDATE products SET created_at = now() - interval '2 years' WHERE id = $1`, dead)
	s.Sweep(t, "approve", nil, func() error { return approve(s.Shop, p, "20") })
	s.Sweep(t, "snooze", nil, func() error { return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/snooze", p), nil, nil) })
	s.Sweep(t, "settings", nil, func() error { return settings(s.Shop, 30, "15", "5") })
	s.Sweep(t, "disable", func() { s.DB.MustExec(`UPDATE products SET is_active = true WHERE id = $1`, dead) },
		func() error {
			return s.Admin.Do("POST", "/api/v1/x/clearance/disable", map[string]any{"productIds": []int64{dead}}, nil)
		})
}

// PLG-7: the data group holds every table.
func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "clearance")
}
