package battle

import (
	"fmt"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// CMB-1, PLG-5, PLG-3: bad requests are refused with a clear 4xx and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	c, a, b := pair(t, s, "Refuse")
	svc := s.Service(t, "Refuse service", "10")
	off := s.Product(t, "Refuse disabled", "10", "0", nil)
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off), nil, nil))
	with := func(f func(m map[string]any)) map[string]any { m := body("Bad", "percent", "10", a, b); f(m); return m }
	before := ltest.State(t, s.DB)
	for _, x := range []struct {
		name string
		do   func() error
	}{
		{"no name", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body(" ", "percent", "10", a, b), nil) }},
		{"percent over 100", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "101", a, b), nil) }},
		{"negative Rs off", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "fixed", "-1", a, b), nil) }},
		{"set price 0", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "price", "0", a, b), nil) }},
		{"unknown deal", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "free", "1", a, b), nil) }},
		{"not a number", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "ten", a, b), nil) }},
		{"one unit", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a), nil) }},
		{"twice the same", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a, a), nil) }},
		{"qty 0", func() error {
			return s.Admin.Do("POST", "/api/v1/x/combos", with(func(m map[string]any) {
				m["items"] = []map[string]any{{"productId": a, "qty": "0"}, {"productId": b, "qty": "2"}}
			}), nil)
		}},
		{"no such product", func() error {
			return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a, 999999), nil)
		}},
		{"a service", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a, svc), nil) }},
		{"a disabled product", func() error { return s.Admin.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a, off), nil) }},
		{"bad date", func() error {
			return s.Admin.Do("POST", "/api/v1/x/combos", with(func(m map[string]any) { m["startsOn"] = "21/11/2026" }), nil)
		}},
		{"ends before it starts", func() error {
			return s.Admin.Do("POST", "/api/v1/x/combos", with(func(m map[string]any) { m["startsOn"], m["endsOn"] = "2026-12-02", "2026-12-01" }), nil)
		}},
		{"edit none", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/combos/999999", body("x", "percent", "10", a, b), nil)
		}},
		{"edit to bad", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/combos/%d", c), body("x", "percent", "200", a, b), nil)
		}},
		{"bad id", func() error { return s.Admin.Do("PUT", "/api/v1/x/combos/abc", body("x", "percent", "10", a, b), nil) }},
		{"switch none", func() error {
			return s.Admin.Do("POST", "/api/v1/x/combos/999999/active", map[string]bool{"active": false}, nil)
		}},
		{"delete none", func() error { return s.Admin.Do("DELETE", "/api/v1/x/combos/999999", nil, nil) }},
		{"cashier creates", func() error { return s.Cashier.Do("POST", "/api/v1/x/combos", body("x", "percent", "10", a, b), nil) }},
		{"cashier settings", func() error {
			return s.Cashier.Do("PUT", "/api/v1/x/combos/settings", map[string]bool{"autoApply": true}, nil)
		}},
		{"bad report date", func() error { return s.Admin.Do("GET", "/api/v1/x/combos/report?from=x&to=y", nil, nil) }},
	} {
		if err := x.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", x.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", x.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-4: a failure at any statement leaves nothing behind — the admin's writes and
// a combo sale (the hook is inside the sale's transaction).
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	openTill(t, s.Shop)
	var c, a, b int64
	fresh := func() { c, a, b = pair(t, s.Shop, "Sweep") }
	s.Sweep(t, "create", fresh, func() error {
		return s.Admin.Do("POST", "/api/v1/x/combos", body("Sweep new", "fixed", "5", a, b), nil)
	})
	s.Sweep(t, "edit", fresh, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/combos/%d", c), body("Sweep edit", "price", "120", a, b), nil)
	})
	s.Sweep(t, "switch off", fresh, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/combos/%d/active", c), map[string]bool{"active": false}, nil)
	})
	s.Sweep(t, "delete", fresh, func() error { return s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/combos/%d", c), nil, nil) })
	s.Sweep(t, "settings", nil, func() error {
		return s.Admin.Do("PUT", "/api/v1/x/combos/settings", map[string]bool{"autoApply": true}, nil)
	})
	s.Sweep(t, "combo sale", fresh, func() error {
		return s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": comboLines(c, a, b, 1, "1"), "payments": []map[string]string{{"method": "cash", "amount": "135"}}}, nil)
	})
}

// PLG-6: a Pay sent twice (same one-time reference) is one sale with one sold row.
func TestPLG6_SaleSentTwice(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c, a, b := pair(t, s, "Twice")
	req := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "clientRef": fmt.Sprintf("twice-%d", c),
		"items": comboLines(c, a, b, 1, "1"), "payments": []map[string]string{{"method": "cash", "amount": "135"}}}
	for range 2 {
		s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", req, nil))
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM combo_sales WHERE combo_id = $1`, c))
	if n != 1 {
		t.Errorf("PLG-6: %d sold rows, want 1", n)
	}
	s.Audit(t)
}

func TestPLG7_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "combos")
}
