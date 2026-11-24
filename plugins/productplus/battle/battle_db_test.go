// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own).
package battle

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/productplus"
)

// Product Plus's battle tests (docs/rules/plugins/productplus.md): one throwaway
// shop on the real server with this plugin compiled in (ltest.Main).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

// field makes a field and returns it (id and key).
func field(t *testing.T, s *ltest.Shop, req map[string]any) productplus.FieldDTO {
	t.Helper()
	body := map[string]any{"showAtTill": true, "searchable": true}
	for k, v := range req {
		body[k] = v
	}
	var f productplus.FieldDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/productplus/fields", body, &f))
	return f
}

func cell(p, f int64, v string) productplus.Cell {
	return productplus.Cell{ProductID: p, FieldID: f, Value: v}
}

func fill(t *testing.T, s *ltest.Shop, cells ...productplus.Cell) productplus.SaveResult {
	t.Helper()
	var r productplus.SaveResult
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/productplus/values", cells, &r))
	return r
}

func value(t *testing.T, s *ltest.Shop, fieldID, productID int64) (string, bool) {
	t.Helper()
	var v []string
	s.Must(t, s.DB.Select(&v, `SELECT value FROM pp_values WHERE field_id = $1 AND product_id = $2`, fieldID, productID))
	if len(v) == 0 {
		return "", false
	}
	return v[0], true
}

// infoRows is the till's ⓘ rows for p, label → value.
func infoRows(t *testing.T, s *ltest.Shop, p int64) map[string]string {
	t.Helper()
	var d struct {
		Rows []struct{ Label, Value string }
	}
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/products/%d/detail", p), nil, &d))
	out := map[string]string{}
	for _, r := range d.Rows {
		out[r.Label] = r.Value
	}
	return out
}

func extra(vals map[string]string) map[string]any {
	return map[string]any{"extra": map[string]any{"productplus": map[string]any{"values": vals}}}
}

func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	n := field(t, s, map[string]any{"label": "Fire number", "type": "number"})
	d := field(t, s, map[string]any{"label": "Fire date", "type": "date"})
	b := field(t, s, map[string]any{"label": "Fire yes", "type": "bool"})
	p := s.Product(t, "Fire pp", "10", "0", nil)
	dup := s.Product(t, "Fire pp dup", "10", "0", nil)
	sup := supplier(t, s, "Fire supplier")
	for _, c := range []struct{ rule, bad string }{
		{"PPF-1", fmt.Sprintf(`INSERT INTO pp_values VALUES (%d, %d, 'NaN')`, n.ID, p)},
		{"PPF-1", fmt.Sprintf(`INSERT INTO pp_values VALUES (%d, %d, '2026-02-30')`, d.ID, p)},
		{"PPF-1", fmt.Sprintf(`INSERT INTO pp_values VALUES (%d, %d, 'yes')`, b.ID, p)},
		{"PPF-6", fmt.Sprintf(`INSERT INTO pp_values VALUES (%d, 999999, '1')`, n.ID)},
		{"PPF-6", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d; INSERT INTO pp_values VALUES (%d, %[2]d, '1')`, p, dup, n.ID)},
		{"PPF-9", fmt.Sprintf(`UPDATE pp_fields SET label = 'FIRE NUMBER' WHERE id = %d`, d.ID)},
		{"PPF-10", fmt.Sprintf(`INSERT INTO pp_supplier_field (field_id) VALUES (%d) ON CONFLICT (id) DO UPDATE SET field_id = EXCLUDED.field_id`, n.ID)},
		{"PPF-11", fmt.Sprintf(`INSERT INTO pp_supplier_map VALUES (' Fire Bosch', %d)`, sup)},
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

// PPF-1, PLG-3: a value that doesn't fit, or for a product or field that doesn't
// exist, is refused on every path and the product keeps what it had.
func TestPPF1_ValuesFit(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	n := field(t, s, map[string]any{"label": "Fit strength", "type": "number"})
	d := field(t, s, map[string]any{"label": "Fit made on", "type": "date"})
	b := field(t, s, map[string]any{"label": "Fit imported", "type": "bool"})
	sel := field(t, s, map[string]any{"label": "Fit size", "type": "select", "options": []string{"Small", "Large"}})
	p := s.Product(t, "Fit tablets", "10", "0", nil)
	fill(t, s, cell(p, n.ID, "500"), cell(p, d.ID, "2026-01-31"), cell(p, sel.ID, "small"))
	if v, _ := value(t, s, sel.ID, p); v != "Small" {
		t.Errorf("PPF-1: dropdown stored %q, want the option's spelling Small", v)
	}
	for _, c := range []productplus.Cell{cell(p, n.ID, "NaN"), cell(p, n.ID, "Inf"), cell(p, n.ID, "0x1p3"), cell(p, n.ID, "five"), cell(p, d.ID, "2026-02-30"),
		cell(p, d.ID, "31/01/2026"), cell(p, b.ID, "maybe"), cell(p, sel.ID, "Huge"), cell(999999, n.ID, "1"), cell(p, 999999, "1")} {
		r := fill(t, s, c)
		if r.Saved != 0 || len(r.Errors) != 1 {
			t.Errorf("PPF-1 / PLG-3: Fill in %+v saved %d, errors %v; want refused", c, r.Saved, r.Errors)
		}
	}
	if v, _ := value(t, s, n.ID, p); v != "500" {
		t.Errorf("PPF-1: number now %q, want 500 kept", v)
	}
	if _, ok := value(t, s, n.ID, 999999); ok {
		t.Errorf("PLG-3: a value was stored for a product that doesn't exist")
	}
	// The product form refuses too, and keeps the product's value.
	err := s.Admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", p), map[string]any{"name": "Fit tablets", "categoryId": s.Cat, "unitId": 1,
		"costPrice": "10", "sellingPrice": "10", "wholesalePrice": "10", "extra": map[string]any{"productplus": map[string]any{"values": map[string]string{n.Key: "Inf"}}}}, nil)
	if err == nil {
		t.Errorf("PPF-1: the product form saved Inf as a number")
	}
	if v, _ := value(t, s, n.ID, p); v != "500" {
		t.Errorf("PPF-1: number after a refused form %q, want 500", v)
	}
	s.Audit(t)
}

// PPF-2, PPF-3, PPF-5: required and scoped fields on the form; a value follows
// the product's category in and out of scope.
func TestPPF2_RequiredAndScope(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	var other struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "PP other"}, &other))
	var sub struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]any{"name": "PP sub", "parentId": s.Cat}, &sub))
	f := field(t, s, map[string]any{"label": "Scope batch code", "type": "text", "required": true, "scope": []int64{s.Cat}})

	body := func(name string, cat int64, vals map[string]string) map[string]any {
		b := map[string]any{"name": name, "categoryId": cat, "unitId": 1, "costPrice": "1", "sellingPrice": "2", "wholesalePrice": "2", "reorderAuto": false}
		if vals != nil {
			b["extra"] = map[string]any{"productplus": map[string]any{"values": vals}}
		}
		return b
	}
	if err := s.Admin.Do("POST", "/api/v1/admin/products", body("Scope missing", s.Cat, map[string]string{f.Key: ""}), nil); err == nil {
		t.Errorf("PPF-2: saved without a required field in scope")
	}
	var in struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/products", body("Scope sub item", sub.ID, map[string]string{f.Key: "B-7"}), &in))
	if v, _ := value(t, s, f.ID, in.ID); v != "B-7" {
		t.Errorf("PPF-3: subcategory product's value %q, want B-7 (a field covers subcategories)", v)
	}
	var out struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/products", body("Scope other item", other.ID, map[string]string{f.Key: "X-1"}), &out))
	if _, ok := value(t, s, f.ID, out.ID); ok {
		t.Errorf("PPF-2: a value was saved for a product outside the field's scope")
	}
	if _, ok := infoRows(t, s, out.ID)["Scope batch code"]; ok {
		t.Errorf("PPF-3: the till shows the field on a product outside its scope")
	}
	if got := infoRows(t, s, in.ID)["Scope batch code"]; got != "B-7" {
		t.Errorf("PPF-3: the till shows %q in scope, want B-7", got)
	}
	// PPF-5: out of scope it's kept but hidden; back in, it shows again.
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", in.ID), body("Scope sub item", other.ID, nil), nil))
	if _, ok := infoRows(t, s, in.ID)["Scope batch code"]; ok {
		t.Errorf("PPF-5: still shown once moved out of scope")
	}
	if v, _ := value(t, s, f.ID, in.ID); v != "B-7" {
		t.Errorf("PPF-5: moving out of scope dropped the value (now %q)", v)
	}
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", in.ID), body("Scope sub item", sub.ID, nil), nil))
	if got := infoRows(t, s, in.ID)["Scope batch code"]; got != "B-7" {
		t.Errorf("PPF-5: moved back, the till shows %q, want B-7", got)
	}
	s.Audit(t)
}

// PPF-4: a lossy type change needs confirming; then what fits converts, the rest goes.
func TestPPF4_TypeChange(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	f := field(t, s, map[string]any{"label": "Change pack size", "type": "text"})
	a := s.Product(t, "Change A", "10", "0", nil)
	b := s.Product(t, "Change B", "10", "0", nil)
	fill(t, s, cell(a, f.ID, "12"), cell(b, f.ID, "a dozen"))
	req := map[string]any{"label": "Change pack size", "type": "number", "showAtTill": true}
	err := s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), req, nil)
	if se, ok := err.(*ltest.StatusError); !ok || se.Code != 409 {
		t.Fatalf("PPF-4: lossy change answered %v, want 409", err)
	}
	if v, _ := value(t, s, f.ID, b); v != "a dozen" {
		t.Errorf("PPF-4: refused change still touched values (%q)", v)
	}
	req["confirmLoss"] = true
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), req, nil))
	if v, _ := value(t, s, f.ID, a); v != "12" {
		t.Errorf("PPF-4: a fitting value became %q, want 12", v)
	}
	if v, ok := value(t, s, f.ID, b); ok {
		t.Errorf("PPF-4: a value that doesn't fit stayed: %q", v)
	}
	s.Audit(t)
}

// PPF-6: a product merge keeps the kept product's values, takes the duplicate's
// only where the kept one had none.
func TestPPF6_Merge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	f := field(t, s, map[string]any{"label": "Merge maker", "type": "text"})
	g := field(t, s, map[string]any{"label": "Merge origin", "type": "text"})
	from := s.Product(t, "Merge pp from", "10", "0", nil)
	into := s.Product(t, "Merge pp into", "10", "0", nil)
	fill(t, s, cell(from, f.ID, "Dup maker"), cell(from, g.ID, "India"), cell(into, f.ID, "Kept maker"))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", from, into), map[string]string{"lotPrice": "follow"}, nil))
	if v, _ := value(t, s, f.ID, into); v != "Kept maker" {
		t.Errorf("PPF-6: kept maker %q, want its own", v)
	}
	if v, _ := value(t, s, g.ID, into); v != "India" {
		t.Errorf("PPF-6: origin %q, want the duplicate's India (the kept one had none)", v)
	}
	s.Audit(t)
}

// PPF-8: a disabled field keeps its values and is hidden; enabled, it's back.
func TestPPF8_DisabledField(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	f := field(t, s, map[string]any{"label": "Hide colourcode", "type": "text"})
	p := s.Product(t, "Hide pen", "10", "0", nil)
	fill(t, s, cell(p, f.ID, "zq-violet-77"))
	found := func() bool {
		var l struct{ Items []struct{ ID int64 } }
		s.Must(t, s.Cashier.Do("GET", "/api/v1/till/products?search=zq-violet-77", nil, &l))
		for _, x := range l.Items {
			if x.ID == p {
				return true
			}
		}
		return false
	}
	if !found() || infoRows(t, s, p)["Hide colourcode"] == "" {
		t.Fatalf("PPF-3: an active searchable field isn't found / shown")
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/active", f.ID), map[string]bool{"active": false}, nil))
	if found() || infoRows(t, s, p)["Hide colourcode"] != "" {
		t.Errorf("PPF-8: a disabled field is still searched or shown")
	}
	var form []productplus.FormFieldDTO
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/x/productplus/form?productId=%d&categoryId=%d", p, s.Cat), nil, &form))
	for _, ff := range form {
		if ff.ID == f.ID {
			t.Errorf("PPF-8: the product form shows a disabled field")
		}
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/active", f.ID), map[string]bool{"active": true}, nil))
	if v, _ := value(t, s, f.ID, p); v != "zq-violet-77" || !found() {
		t.Errorf("PPF-8: enabled again, value %q / found %v", v, found())
	}
	s.Audit(t)
}

// PLG-5, PPF-9: bad field requests are refused and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	f := field(t, s, map[string]any{"label": "Refuse label", "type": "text"})
	before := ltest.State(t, s.DB)
	for _, c := range []struct {
		name string
		body map[string]any
		path string
	}{
		{"same label", map[string]any{"label": "refuse LABEL", "type": "text"}, ""},
		{"a core column", map[string]any{"label": "Name", "type": "text"}, ""},
		{"no label", map[string]any{"label": " ", "type": "text"}, ""},
		{"bad type", map[string]any{"label": "Refuse type", "type": "colour"}, ""},
		{"dropdown, no options", map[string]any{"label": "Refuse dd", "type": "select"}, ""},
		{"update none", map[string]any{"label": "Refuse none", "type": "text"}, "/999999"},
	} {
		method := "POST"
		if c.path != "" {
			method = "PUT"
		}
		err := s.Admin.Do(method, "/api/v1/x/productplus/fields"+c.path, c.body, nil)
		if err == nil {
			t.Errorf("PLG-5 %s: accepted", c.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", c.name, err)
		}
	}
	for name, do := range map[string]func() error{
		"cashier makes a field": func() error {
			return s.Cashier.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": "Refuse c", "type": "text"}, nil)
		},
		"disable none": func() error {
			return s.Admin.Do("POST", "/api/v1/x/productplus/fields/999999/active", map[string]bool{"active": false}, nil)
		},
		"move none": func() error {
			return s.Admin.Do("POST", "/api/v1/x/productplus/fields/999999/move", map[string]string{"dir": "up"}, nil)
		},
		"rename to another's label": func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), map[string]any{"label": "Fit strength", "type": "text"}, nil)
		},
	} {
		if name == "rename to another's label" {
			field(t, s, map[string]any{"label": "Refuse other", "type": "text"})
			before = ltest.State(t, s.DB)
			do = func() error {
				return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), map[string]any{"label": "Refuse other", "type": "text"}, nil)
			}
		}
		if err := do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-1: fields and values move no money or stock.
func TestPLG1_NothingMoves(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Still pp", "10", "4", nil)
	before := ltest.State(t, s.DB)
	f := field(t, s, map[string]any{"label": "Still field", "type": "number"})
	fill(t, s, cell(p, f.ID, "3"))
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), map[string]any{"label": "Still field", "type": "text"}, nil))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/active", f.ID), map[string]bool{"active": false}, nil))
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
}

// PLG-4: a failure at any statement of a write leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	p := s.Product(t, "Sweep pp", "10", "0", nil)
	n := 0
	s.Sweep(t, "make a field", nil, func() error {
		n++
		return s.Admin.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": fmt.Sprintf("Sweep f%d", n), "type": "select",
			"options": []string{"A", "B"}, "scope": []int64{s.Cat}}, nil)
	})
	f := field(t, s.Shop, map[string]any{"label": "Sweep text", "type": "text"})
	g := field(t, s.Shop, map[string]any{"label": "Sweep text 2", "type": "text"})
	s.Sweep(t, "fill in", nil, func() error {
		return s.Admin.Do("PUT", "/api/v1/x/productplus/values", []productplus.Cell{cell(p, f.ID, "one"), cell(p, g.ID, "two")}, nil)
	})
	s.Sweep(t, "lossy type change", func() { fill(t, s.Shop, cell(p, f.ID, "words")) }, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/productplus/fields/%d", f.ID), map[string]any{"label": "Sweep text", "type": "number", "confirmLoss": true}, nil)
	})
	s.Sweep(t, "move", nil, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/productplus/fields/%d/move", g.ID), map[string]string{"dir": "up"}, nil)
	})
	s.Sweep(t, "the product form's values", nil, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", p), map[string]any{"name": "Sweep pp", "categoryId": s.Cat, "unitId": 1,
			"costPrice": "10", "sellingPrice": "10", "wholesalePrice": "10", "extra": map[string]any{"productplus": map[string]any{"values": map[string]string{g.Key: strings.Repeat("x", 3)}}}}, nil)
	})
}

func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "productplus")
}
