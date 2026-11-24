package battle

import (
	"fmt"
	"slices"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/productplus"
)

// Supplier by field (PPF-10…13, SUP-12). The supplier field is one per shop, so
// these tests don't run in parallel (Go runs them before the parallel ones) and
// each clears the field when it's done.

func supplier(t *testing.T, s *ltest.Shop, name string) int64 {
	t.Helper()
	var r struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/suppliers", map[string]any{"name": name}, &r))
	return r.ID
}

func chooseField(s *ltest.Shop, id int64, confirm bool) error {
	return s.Admin.Do("PUT", "/api/v1/x/productplus/supplier-rules/field", map[string]any{"fieldId": id, "confirm": confirm}, nil)
}

func setRule(t *testing.T, s *ltest.Shop, value string, sup int64) {
	t.Helper()
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/productplus/supplier-rules/value", map[string]any{"value": value, "supplierId": sup}, nil))
}

func rules(t *testing.T, s *ltest.Shop) productplus.SupplierRulesDTO {
	t.Helper()
	var r productplus.SupplierRulesDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/productplus/supplier-rules", nil, &r))
	return r
}

// bySupplier is the product ids on the products list filtered by sup, and its total.
func bySupplier(t *testing.T, s *ltest.Shop, sup int64) ([]int64, int) {
	t.Helper()
	var l struct {
		Items []struct{ ID int64 }
		Total int
	}
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/products?supplier=%d", sup), nil, &l))
	var ids []int64
	for _, it := range l.Items {
		ids = append(ids, it.ID)
	}
	return ids, l.Total
}

func clearField(t *testing.T, s *ltest.Shop) {
	t.Cleanup(func() { _ = chooseField(s, 0, true) })
}

func TestPPF10_OneUsableField(t *testing.T) {
	s := ltest.Shared(t).Shop
	clearField(t, s)
	num := field(t, s, map[string]any{"label": "Sup num", "type": "number"})
	brand := field(t, s, map[string]any{"label": "Sup brand A", "type": "text"})
	maker := field(t, s, map[string]any{"label": "Sup maker A", "type": "select", "options": []string{"X", "Y"}})
	if err := chooseField(s, num.ID, false); err == nil {
		t.Errorf("PPF-10: a number field was chosen")
	}
	s.Must(t, chooseField(s, brand.ID, false))
	setRule(t, s, "Bosch", supplier(t, s, "PPF10 sup"))
	if err := chooseField(s, maker.ID, false); err == nil {
		t.Errorf("PPF-10: switched field with rules, no confirm")
	}
	s.Must(t, chooseField(s, maker.ID, true))
	r := rules(t, s)
	if r.FieldID != maker.ID {
		t.Errorf("PPF-10: field %d, want %d", r.FieldID, maker.ID)
	}
	for _, v := range r.Values {
		if v.SupplierID != 0 {
			t.Errorf("PPF-10: switching kept the rule for %q", v.Value)
		}
	}
	if slices.ContainsFunc(r.Fields, func(f productplus.FieldOption) bool { return f.ID == num.ID }) {
		t.Errorf("PPF-10: a number field is offered")
	}
	s.Audit(t)
}

func TestPPF11_RuleSupplierInLists(t *testing.T) {
	s := ltest.Shared(t).Shop
	clearField(t, s)
	var other struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "PPF11 other"}, &other))
	brand := field(t, s, map[string]any{"label": "Sup brand B", "type": "text", "scope": []int64{s.Cat}})
	s1, s9 := supplier(t, s, "PPF11 S1"), supplier(t, s, "PPF11 S9")
	p := func(name, v string) int64 {
		return s.Product(t, name, "20", "0", extra(map[string]string{brand.Key: v}))
	}
	p1, p2, p3, p4 := p("PPF11 one", "Bosch "), p("PPF11 two", "BOSCH"), p("PPF11 three", "bosch"), p("PPF11 four", "NGK")
	s.Must(t, chooseField(s, brand.ID, false))
	setRule(t, s, "bosch", s1)

	ids, total := bySupplier(t, s, s1)
	for _, id := range []int64{p1, p2, p3} {
		if !slices.Contains(ids, id) {
			t.Errorf("PPF-11: product %d (Bosch, any spelling) missing from S1", id)
		}
	}
	if slices.Contains(ids, p4) || total != 3 {
		t.Errorf("PPF-11: S1 lists %v (total %d), want the three Bosch", ids, total)
	}
	// Its own supplier wins.
	s.DB.MustExec(`UPDATE products SET preferred_supplier_id = $1 WHERE id = $2`, s9, p2)
	if ids, _ = bySupplier(t, s, s1); slices.Contains(ids, p2) {
		t.Errorf("SUP-12: a product with its own supplier still listed by rule")
	}
	// A disabled field gives none; enabled, back.
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/active", brand.ID), map[string]any{"active": false}, nil))
	if ids, _ = bySupplier(t, s, s1); len(ids) != 0 {
		t.Errorf("PPF-11: field disabled, S1 still lists %v", ids)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/active", brand.ID), map[string]any{"active": true}, nil))
	if ids, _ = bySupplier(t, s, s1); !slices.Contains(ids, p1) {
		t.Errorf("PPF-11: field enabled again, rule not back")
	}
	// Out of the field's scope: no rule supplier.
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", p3), map[string]any{"name": "PPF11 three", "categoryId": other.ID,
		"unitId": 1, "costPrice": "10", "sellingPrice": "20", "wholesalePrice": "20", "reorderAuto": false}, nil))
	if ids, _ = bySupplier(t, s, s1); slices.Contains(ids, p3) {
		t.Errorf("PPF-11: a product out of the field's scope still gets the rule")
	}
	s.Audit(t)
}

func TestPPF11_DisabledSupplier(t *testing.T) {
	s := ltest.Shared(t).Shop
	clearField(t, s)
	brand := field(t, s, map[string]any{"label": "Sup brand C", "type": "text"})
	s1 := supplier(t, s, "PPF11d S1")
	p1 := s.Product(t, "PPF11d one", "20", "0", extra(map[string]string{brand.Key: "Denso"}))
	s.Must(t, chooseField(s, brand.ID, false))
	setRule(t, s, "Denso", s1)
	detail := func() (int64, string) {
		var d struct {
			SupplierID       int64
			RuleSupplierName string
		}
		s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/products/%d", p1), nil, &d))
		return d.SupplierID, d.RuleSupplierName
	}
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/suppliers/%d", s1), nil, nil))
	if ids, _ := bySupplier(t, s, s1); len(ids) != 0 {
		t.Errorf("SUP-12: a disabled supplier still gets %v by rule", ids)
	}
	if _, n := detail(); n != "" {
		t.Errorf("SUP-12: a disabled supplier shown as the rule supplier %q", n)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/activate", s1), nil, nil))
	if ids, _ := bySupplier(t, s, s1); !slices.Contains(ids, p1) {
		t.Errorf("SUP-12: supplier back, rule products not back")
	}
	if own, n := detail(); own != 0 || n != "PPF11d S1" {
		t.Errorf("SUP-12: product detail own %d rule %q", own, n)
	}
	s.Audit(t)
}

func TestPPF12_ValueList(t *testing.T) {
	s := ltest.Shared(t).Shop
	clearField(t, s)
	maker := field(t, s, map[string]any{"label": "Sup maker D", "type": "select", "options": []string{"Bosch", "bosch ", "NGK", "Unused"}})
	sup := supplier(t, s, "PPF12 sup")
	for i, v := range []string{"Bosch", "Bosch", "bosch ", "NGK"} {
		s.Product(t, fmt.Sprintf("PPF12 p%d", i), "20", "0", extra(map[string]string{maker.Key: v}))
	}
	s.Must(t, chooseField(s, maker.ID, false))
	setRule(t, s, "NGK", sup)
	r := rules(t, s)
	got := map[string]productplus.SupplierValueDTO{}
	var order []string
	for _, v := range r.Values {
		got[v.Value] = v
		order = append(order, v.Value)
	}
	if b, ok := got["Bosch"]; !ok || b.Products != 3 {
		t.Errorf("PPF-12: Bosch row %+v, want one row with 3 products (rows %v)", b, order)
	}
	if _, ok := got["bosch "]; ok {
		t.Errorf("PPF-12: a second spelling got its own row")
	}
	if u, ok := got["Unused"]; !ok || u.Products != 0 {
		t.Errorf("PPF-12: unused option row %+v", u)
	}
	if n := got["NGK"]; n.SupplierID != sup || n.SupplierName != "PPF12 sup" {
		t.Errorf("PPF-12: NGK row %+v", n)
	}
	if len(order) < 3 || order[len(order)-1] != "NGK" || order[0] != "Bosch" {
		t.Errorf("PPF-12: order %v, want no-supplier first (most products first), NGK last", order)
	}
}
