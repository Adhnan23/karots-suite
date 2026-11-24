package products

import (
	"context"
	"slices"
	"testing"
)

// SUP-12: a product's supplier is its own, else a plugin's rule supplier (a
// disabled one counts as none), else none. Core keeps the rule supplier apart.
type ruleFixture struct {
	s1, s2, off, a, b, c int64
}

func ruleSetup(t *testing.T) (*Service, ruleFixture) {
	t.Helper()
	conn := ppTestDB(t)
	t.Cleanup(func() { conn.Close() })
	ctx := context.Background()
	var f ruleFixture
	var unit, cat int64
	must(t, conn.GetContext(ctx, &unit, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ('TEST rule sup cat') RETURNING id`))
	for _, s := range []struct {
		id     *int64
		name   string
		active bool
	}{{&f.s1, "TEST rule S1", true}, {&f.s2, "TEST rule S2", true}, {&f.off, "TEST rule off", false}} {
		must(t, conn.GetContext(ctx, s.id, `INSERT INTO suppliers (name, is_active) VALUES ($1, $2) RETURNING id`, s.name, s.active))
	}
	mk := func(name string, own *int64) int64 {
		var id int64
		must(t, conn.GetContext(ctx, &id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, preferred_supplier_id)
			VALUES ($1, $2, $3, 10, 20, $4) RETURNING id`, name, cat, unit, own))
		return id
	}
	f.a, f.b, f.c = mk("TEST rule A", &f.s1), mk("TEST rule B", nil), mk("TEST rule C", nil)
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1,$2,$3)`, f.a, f.b, f.c)    //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM suppliers WHERE id IN ($1,$2,$3)`, f.s1, f.s2, f.off) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, cat)                      //nolint:errcheck
	})
	return NewService(conn), f
}

// fakeRules: A and B by rule to s2 (A has its own, so it must not count), C to a
// disabled supplier. asked records the ids For was called with.
func fakeRules(t *testing.T, f ruleFixture, asked *[]int64) {
	t.Helper()
	rules := map[int64]RuleSupplierOf{f.a: {SupplierID: f.s2, Why: "Brand: Bosch"}, f.b: {SupplierID: f.s2, Why: "Brand: Bosch"},
		f.c: {SupplierID: f.off, Why: "Brand: NGK"}}
	RuleSupplierFor = func(_ context.Context, ids []int64) (map[int64]RuleSupplierOf, error) {
		*asked = append(*asked, ids...)
		out := map[int64]RuleSupplierOf{}
		for _, id := range ids {
			if r, ok := rules[id]; ok {
				out[id] = r
			}
		}
		return out, nil
	}
	RuleSupplierProducts = func(_ context.Context, sup int64) ([]int64, error) {
		var out []int64
		for id, r := range rules {
			if r.SupplierID == sup {
				out = append(out, id)
			}
		}
		return out, nil
	}
	t.Cleanup(func() { RuleSupplierFor, RuleSupplierProducts = nil, nil })
}

func byID(rows []Product) map[int64]Product {
	m := map[int64]Product{}
	for _, p := range rows {
		m[p.ID] = p
	}
	return m
}

func TestSUP12_ListFillsRuleSupplier(t *testing.T) {
	svc, f := ruleSetup(t)
	var asked []int64
	fakeRules(t, f, &asked)
	rows, _, err := svc.List(context.Background(), ListQuery{Search: "TEST rule", Limit: 50, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	m := byID(rows)
	if m[f.a].RuleSupplierID != nil {
		t.Errorf("A has its own supplier, got a rule supplier")
	}
	if b := m[f.b]; b.RuleSupplierID == nil || *b.RuleSupplierID != f.s2 || b.RuleSupplierName != "TEST rule S2" || b.RuleSupplierWhy != "Brand: Bosch" {
		t.Errorf("B: got %v %q %q", b.RuleSupplierID, b.RuleSupplierName, b.RuleSupplierWhy)
	}
	if m[f.c].RuleSupplierID != nil {
		t.Errorf("C's rule supplier is disabled: want none")
	}
	if slices.Contains(asked, f.a) || !slices.Contains(asked, f.b) || !slices.Contains(asked, f.c) {
		t.Errorf("For asked %v: want B and C only", asked)
	}
}

func TestSUP12_FilterBySupplierIncludesRule(t *testing.T) {
	svc, f := ruleSetup(t)
	var asked []int64
	fakeRules(t, f, &asked)
	ctx := context.Background()
	rows, total, err := svc.List(ctx, ListQuery{PreferredSupplierID: &f.s2, Limit: 50, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	m := byID(rows)
	if _, ok := m[f.b]; !ok {
		t.Errorf("S2's filter misses B (by rule)")
	}
	if _, ok := m[f.a]; ok {
		t.Errorf("S2's filter has A, whose own supplier is S1")
	}
	if total != len(rows) {
		t.Errorf("total %d, rows %d", total, len(rows))
	}
	rows, _, err = svc.List(ctx, ListQuery{PreferredSupplierID: &f.off, Limit: 50, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := byID(rows)[f.c]; ok {
		t.Errorf("a disabled supplier's filter has C by rule")
	}
	sheet, err := svc.repo.CountSheet(ctx, nil, &f.s2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(sheet, func(r CountSheetRow) bool { return r.ID == f.b }) {
		t.Errorf("count sheet for S2 misses B")
	}
}

func TestSUP12_GetFillsRuleSupplier(t *testing.T) {
	svc, f := ruleSetup(t)
	var asked []int64
	fakeRules(t, f, &asked)
	p, err := svc.Get(context.Background(), f.b)
	if err != nil {
		t.Fatal(err)
	}
	if p.RuleSupplierID == nil || *p.RuleSupplierID != f.s2 || p.PreferredSupplierID != nil {
		t.Errorf("B: rule %v own %v", p.RuleSupplierID, p.PreferredSupplierID)
	}
}
