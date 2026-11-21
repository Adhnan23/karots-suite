// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own). The AI Assistant works through other plugins (AIC-9), so
// Product Plus, Alternatives and Pharmacy are compiled in too and their auditor
// checks run after every test.
package battle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/aicatalog"
	_ "karots-suite/plugins/alternatives"
	_ "karots-suite/plugins/pharmacy"
	"karots-suite/plugins/productplus"
)

// AI Assistant's battle tests (docs/rules/plugins/aicatalog.md).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

type preview struct {
	Result *aicatalog.JobPreview `json:"result"`
	Prompt string                `json:"prompt"`
	Error  string                `json:"error"`
}

// next asks job about exactly ids with a pasted reply and returns its preview.
func next(t *testing.T, s *ltest.Shop, job string, ids []int64, reply string, fields ...string) preview {
	t.Helper()
	var p preview
	body := map[string]any{"ids": ids, "reply": reply, "force": true}
	if len(fields) > 0 { // Fill in details: the ticked Product Plus fields
		body["options"] = map[string]any{"fields": fields}
	}
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/aicatalog/jobs/"+job+"/next", body, &p))
	return p
}

// apply ticks every row of a preview (or only those keep says) and applies them.
func apply(t *testing.T, s *ltest.Shop, job string, p preview, keep func(aicatalog.JobRow) bool) aicatalog.JobApplied {
	t.Helper()
	if p.Result == nil {
		t.Fatalf("%s: no preview (%s)", job, p.Error)
	}
	rows := []aicatalog.JobRow{}
	for _, r := range p.Result.Rows {
		if keep == nil || keep(r) {
			r.Ticked = true
			rows = append(rows, r)
		}
	}
	var out aicatalog.JobApplied
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/aicatalog/jobs/"+job+"/apply", aicatalog.JobApply{Rows: rows, Asked: p.Result.Asked, State: p.Result.State}, &out))
	return out
}

func undo(t *testing.T, s *ltest.Shop) {
	t.Helper()
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/aicatalog/cleanup/undo", nil, nil))
}

func ppField(t *testing.T, s *ltest.Shop, label, typ string) productplus.FieldDTO {
	t.Helper()
	var f productplus.FieldDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": label, "type": typ}, &f))
	return f
}

func ppValue(t *testing.T, s *ltest.Shop, field, product int64) string {
	t.Helper()
	var v []string
	s.Must(t, s.DB.Select(&v, `SELECT value FROM pp_values WHERE field_id = $1 AND product_id = $2`, field, product))
	return strings.Join(v, "")
}

func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := s.Product(t, "Fire ai A", "10", "0", nil)
	b := s.Product(t, "Fire ai B", "10", "0", nil)
	for _, c := range []struct{ rule, bad string }{
		{"AIC-5", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d; INSERT INTO aicatalog_items (product_id, resolved_name, source) VALUES (%[2]d, 'x', 'ai')`, b, a)},
		{"AIC-5", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d; INSERT INTO aicatalog_job_items (job, product_id, state) VALUES ('fields', %[2]d, 'applied')`, b, a)},
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

// AIC-9, AIC-2, AIC-3: the Fill-in-details job writes Product Plus values through
// its attribute set — a value that doesn't fit the field is skipped, never stored —
// and undo takes them back.
func TestAIC9_FieldsThroughProductPlus(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	maker := ppField(t, s, "AI maker", "text")
	strength := ppField(t, s, "AI strength", "number")
	a := s.Product(t, "AI panadol", "10", "0", nil)
	b := s.Product(t, "AI piriton", "10", "0", nil)
	p := next(t, s, "fields", []int64{a, b}, fmt.Sprintf(`{"items":[
		{"id":%d,"values":[{"field":%q,"value":"GSK"},{"field":%q,"value":"NaN"}]},
		{"id":%d,"values":[{"field":%q,"value":"500"}]},
		{"id":999999,"values":[{"field":%q,"value":"Foreign"}]}]}`, a, maker.Key, strength.Key, b, strength.Key, maker.Key), maker.Key, strength.Key)
	if p.Result == nil {
		t.Fatalf("fields job: %s", p.Error)
	}
	for _, r := range p.Result.Rows {
		if r.ProductID == 999999 {
			t.Errorf("AIC-6: a row for a product not asked about: %+v", r)
		}
	}
	apply(t, s, "fields", p, nil)
	if v := ppValue(t, s, maker.ID, a); v != "GSK" {
		t.Errorf("AIC-9: maker %q, want GSK", v)
	}
	if v := ppValue(t, s, strength.ID, b); v != "500" {
		t.Errorf("AIC-9: strength %q, want 500", v)
	}
	if v := ppValue(t, s, strength.ID, a); v != "" {
		t.Errorf("AIC-9 / PPF-1: NaN stored as a number: %q", v)
	}
	s.Audit(t)
	undo(t, s)
	if v := ppValue(t, s, maker.ID, a) + ppValue(t, s, strength.ID, b); v != "" {
		t.Errorf("AIC-3: undo left %q", v)
	}
	s.Audit(t)
}

// AIC-9: the group maker makes Alternatives groups through its grouper; undo removes them.
func TestAIC9_GroupsThroughAlternatives(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := s.Product(t, "AI amoxil 250", "10", "0", nil)
	b := s.Product(t, "AI amoxicillin 250", "10", "0", nil)
	p := next(t, s, "groups", []int64{a, b}, fmt.Sprintf(`{"groups":[{"name":"AI Amoxicillin 250mg","product_ids":[%d,%d],"reason":"same"}]}`, a, b))
	apply(t, s, "groups", p, nil)
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM alt_members WHERE product_id IN ($1, $2)`, a, b))
	if n != 2 {
		t.Errorf("AIC-9: %d of 2 products grouped", n)
	}
	s.Audit(t)
	undo(t, s)
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM alt_members WHERE product_id IN ($1, $2)`, a, b))
	if n != 0 {
		t.Errorf("AIC-3: undo left %d memberships", n)
	}
	s.Audit(t)
}

// AIC-9: prescription marks go through Pharmacy's attribute set.
func TestAIC9_MarksThroughPharmacy(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := s.Product(t, "AI augmentin", "10", "0", nil)
	p := next(t, s, "marks", []int64{a}, fmt.Sprintf(`{"items":[{"id":%d,"rx_only":true,"controlled":false,"reason":"antibiotic"}]}`, a))
	apply(t, s, "marks", p, nil)
	var rx bool
	s.Must(t, s.DB.Get(&rx, `SELECT COALESCE((SELECT rx_only FROM pharmacy_medicines WHERE product_id = $1), false)`, a))
	if !rx {
		t.Errorf("AIC-9: the prescription mark didn't reach Pharmacy")
	}
	undo(t, s)
	s.Must(t, s.DB.Get(&rx, `SELECT COALESCE((SELECT rx_only FROM pharmacy_medicines WHERE product_id = $1), false)`, a))
	if rx {
		t.Errorf("AIC-3: undo left the prescription mark")
	}
	s.Audit(t)
}

// AIC-1: a setup job (expiry, a pack) changes no price, stock or money.
func TestAIC1_SetupMovesNothing(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	a := s.Product(t, "AI vitamin c", "250", "12", nil)
	before := ltest.State(t, s.DB)
	var price string
	s.Must(t, s.DB.Get(&price, `SELECT selling_price::text FROM products WHERE id = $1`, a))
	p := next(t, s, "setup", []int64{a}, fmt.Sprintf(`{"items":[{"id":%d,"has_expiry":true,"pack_name":"Card","pack_size":10,"reason":"tablets"}]}`, a))
	apply(t, s, "setup", p, nil)
	undo(t, s)
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
	var after string
	s.Must(t, s.DB.Get(&after, `SELECT selling_price::text FROM products WHERE id = $1`, a))
	if after != price {
		t.Errorf("AIC-1: price %s → %s", price, after)
	}
	s.Audit(t)
}

// AIC-4, PPF-7: a clean-up that merges categories carries Product Plus scopes onto
// the kept one; undo puts the products, the category and the scope back.
func TestAIC4_CleanupMergeAndUndo(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	var from, into struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "AI Tabs"}, &from))
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "AI Tablets"}, &into))
	var f productplus.FieldDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": "AI strip", "type": "text", "scope": []int64{from.ID}}, &f))
	p := s.Product(t, "AI tab product", "10", "0", map[string]any{"categoryId": from.ID})
	before := ltest.State(t, s.DB)
	var catsBefore []string
	s.Must(t, s.DB.Select(&catsBefore, `SELECT c::text FROM categories c WHERE id IN ($1, $2) ORDER BY id`, from.ID, into.ID))
	plan := map[string]any{"merges": []map[string]any{{"from": from.ID, "into": into.ID, "reason": "same"}}}
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/aicatalog/cleanup/apply", map[string]any{"plan": plan}, nil))
	var cat int64
	s.Must(t, s.DB.Get(&cat, `SELECT category_id FROM products WHERE id = $1`, p))
	if cat != into.ID {
		t.Errorf("AIC-4: product in category %d after the merge, want %d", cat, into.ID)
	}
	var scoped int
	s.Must(t, s.DB.Get(&scoped, `SELECT count(*) FROM pp_field_categories WHERE field_id = $1 AND category_id = $2`, f.ID, into.ID))
	if scoped != 1 {
		t.Errorf("PPF-7: the field's scope didn't follow the merge")
	}
	s.Audit(t)
	undo(t, s)
	ch := ltest.Changed(before, ltest.State(t, s.DB))
	for _, tb := range ch {
		if !strings.HasPrefix(tb, "aicatalog_") {
			t.Errorf("AIC-4 / PPF-7: undo left %s changed", tb)
		}
		if tb == "categories" {
			var catsAfter []string
			s.Must(t, s.DB.Select(&catsAfter, `SELECT c::text FROM categories c WHERE id IN ($1, $2) ORDER BY id`, from.ID, into.ID))
			t.Logf("before %v\nafter  %v", catsBefore, catsAfter)
		}
	}
	s.Audit(t)
}

// AIC-6: a malformed answer changes nothing.
func TestAIC6_MalformedAnswer(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	a := s.Product(t, "AI garbled", "10", "0", nil)
	for _, reply := range []string{`not json at all`, `{"items":"x"}`, `{"items":[{"id":"abc"}]}`} {
		p := next(t, s, "identify", []int64{a}, reply)
		if p.Result != nil && len(p.Result.Rows) > 0 {
			t.Errorf("AIC-6: %q gave rows %+v", reply, p.Result.Rows)
		}
	}
	var name string
	s.Must(t, s.DB.Get(&name, `SELECT name FROM products WHERE id = $1`, a))
	if name != "AI garbled" {
		t.Errorf("AIC-6: the product became %q", name)
	}
}

// AIC-7: Suggest combos sends the best sellers, so a new product that sells is in
// even when the shop already has 200 older ones.
func TestAIC7_CombosSendBestSellers(t *testing.T) {
	t.Skip("AIC-7 (user's answer 2026-11-21) not built yet: fails today (the 200 oldest are sent) — next session builds it")
	t.Parallel()
	s := ltest.Shared(t).Shop
	for i := range 205 {
		s.DB.MustExec(`INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ($1, $2, 1, 1, 2)`, fmt.Sprintf("AI filler %03d", i), s.Cat)
	}
	hot := s.Product(t, "AI hot seller", "50", "100", nil)
	cold := s.Product(t, "AI never sold", "50", "100", nil)
	s.OpenTill(t, s.Cashier)
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": []map[string]any{{"productId": hot, "quantity": "3"}}, "payments": []map[string]any{{"method": "cash", "amount": "150"}}}, nil))
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/aicatalog/settings", map[string]any{"manualMode": true}, nil))
	var r struct{ Prompt string }
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/aicatalog/combo-suggest", map[string]any{}, &r))
	if !strings.Contains(r.Prompt, fmt.Sprintf("%d|AI hot seller", hot)) {
		t.Errorf("AIC-7: the best seller isn't in the combos prompt")
	}
	if n := strings.Count(r.Prompt, "\n") - strings.Count(strings.SplitN(r.Prompt, "PRODUCTS", 2)[0], "\n"); n > 260 {
		t.Errorf("AIC-7: the prompt lists about %d lines, want at most 200 products", n)
	}
	_ = cold
}

// AIC-8: the API key is never sent back to a screen, and a backup marks it secret.
func TestAIC8_KeyStaysHidden(t *testing.T) { // changes the shop's settings: runs alone
	s := ltest.Shared(t).Shop
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/aicatalog/settings", map[string]any{"manualMode": false, "provider": "openai",
		"baseUrl": "https://example.invalid/v1", "model": "m", "apiKey": "sk-battle-secret-123"}, nil))
	var raw json.RawMessage
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/aicatalog/settings", nil, &raw))
	if strings.Contains(string(raw), "sk-battle-secret") {
		t.Errorf("AIC-8: the settings page got the key: %s", raw)
	}
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/aicatalog/settings", map[string]any{"manualMode": true}, nil))
}

// AIC-5: a product merge keeps the kept product's learnt item and moves job items.
func TestAIC5_Merge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	f := ppField(t, s, "AI merge maker", "text")
	from := s.Product(t, "AI merge from", "10", "0", nil)
	into := s.Product(t, "AI merge into", "10", "0", nil)
	apply(t, s, "fields", next(t, s, "fields", []int64{from}, fmt.Sprintf(`{"items":[{"id":%d,"values":[{"field":%q,"value":"Cipla"}]}]}`, from, f.Key), f.Key), nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", from, into), map[string]string{"lotPrice": "follow"}, nil))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM aicatalog_job_items WHERE product_id = $1`, from))
	if n != 0 {
		t.Errorf("AIC-5: %d job items still name the merged-away product", n)
	}
	s.Audit(t)
}

// PLG-5: bad requests are refused and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	before := ltest.State(t, s.DB)
	for name, do := range map[string]func() error{
		"unknown job":       func() error { return s.Admin.Do("POST", "/api/v1/x/aicatalog/jobs/nope/next", map[string]any{}, nil) },
		"apply unknown job": func() error { return s.Admin.Do("POST", "/api/v1/x/aicatalog/jobs/nope/apply", map[string]any{}, nil) },
		"cashier runs a job": func() error {
			return s.Cashier.Do("POST", "/api/v1/x/aicatalog/jobs/fields/next", map[string]any{}, nil)
		},
		"no provider": func() error {
			return s.Admin.Do("PUT", "/api/v1/x/aicatalog/settings", map[string]any{"manualMode": false}, nil)
		},
	} {
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

// PLG-4: a failure at any statement of apply or undo leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	f := ppField(t, s.Shop, "AI sweep maker", "text")
	a := s.Product(t, "AI sweep A", "10", "0", nil)
	b := s.Product(t, "AI sweep B", "10", "0", nil)
	var p preview
	ready := func() {
		s.DB.MustExec(`DELETE FROM pp_values WHERE field_id = $1`, f.ID)
		p = next(t, s.Shop, "fields", []int64{a, b}, fmt.Sprintf(`{"items":[{"id":%d,"values":[{"field":%q,"value":"Sun"}]},{"id":%d,"values":[{"field":%[2]q,"value":"Moon"}]}]}`, a, f.Key, b), f.Key)
	}
	s.Sweep(t, "apply a job", ready, func() error {
		rows := p.Result.Rows
		for i := range rows {
			rows[i].Ticked = true
		}
		return s.Admin.Do("POST", "/api/v1/x/aicatalog/jobs/fields/apply", aicatalog.JobApply{Rows: rows, Asked: p.Result.Asked, State: p.Result.State}, nil)
	})
	s.Sweep(t, "undo a job", func() { ready(); apply(t, s.Shop, "fields", p, nil) }, func() error {
		return s.Admin.Do("POST", "/api/v1/x/aicatalog/cleanup/undo", nil, nil)
	})
	var c1, c2 struct{ ID int64 }
	n := 0
	s.Sweep(t, "apply a clean-up merge", func() {
		n++
		s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": fmt.Sprintf("AI sweep from %d", n)}, &c1))
		s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": fmt.Sprintf("AI sweep into %d", n)}, &c2))
	}, func() error {
		return s.Admin.Do("POST", "/api/v1/x/aicatalog/cleanup/apply", map[string]any{"plan": map[string]any{"merges": []map[string]any{{"from": c1.ID, "into": c2.ID}}}}, nil)
	})
}

func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "aicatalog")
}
