package battle

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"testing"
	"time"

	"karots-suite/internal/features/reorder"
	"karots-suite/internal/ledgercheck/ltest"
)

// SUP-12 at core's read sites: where a rule supplier counts, and where it never does.

// ruleShop: a Brand field chosen, rule bosch → S1, P1 Bosch with no own supplier,
// low on stock (1 on hand, level 5).
func ruleShop(t *testing.T, tag string) (s *ltest.Shop, s1, p1 int64) {
	t.Helper()
	s = ltest.Shared(t).Shop
	clearField(t, s)
	brand := field(t, s, map[string]any{"label": "Sup brand " + tag, "type": "text"})
	s1 = supplier(t, s, tag+" S1")
	ex := extra(map[string]string{brand.Key: "Bosch"})
	ex["reorderLevel"] = 5
	p1 = s.Product(t, tag+" one", "20", "1", ex)
	s.Must(t, chooseField(s, brand.ID, false))
	setRule(t, s, "bosch", s1)
	return s, s1, p1
}

type reorderItem struct {
	ProductID    int64
	SupplierID   int64
	SupplierName string
	SupplierWhy  string
}

func lowStock(t *testing.T, s *ltest.Shop, sup int64) map[int64]reorderItem {
	t.Helper()
	var l struct{ Items []reorderItem }
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/reorder?supplier=%d&all=1", sup), nil, &l))
	out := map[int64]reorderItem{}
	for _, it := range l.Items {
		out[it.ProductID] = it
	}
	return out
}

func TestSUP12_LowStockAndOrder(t *testing.T) {
	s, s1, p1 := ruleShop(t, "SUP12a")
	it, ok := lowStock(t, s, s1)[p1]
	if !ok || it.SupplierID != s1 || it.SupplierName != "SUP12a S1" || it.SupplierWhy != "Sup brand SUP12a: Bosch" {
		t.Fatalf("SUP-12: Low stock row %+v (listed %v)", it, ok)
	}
	var ids struct{ IDs []int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/purchases/draft", map[string]any{"lines": []map[string]any{
		{"productId": p1, "supplierId": it.SupplierID, "quantity": "10", "costPrice": "10"}}}, &ids))
	var own *int64
	s.Must(t, s.DB.Get(&own, `SELECT preferred_supplier_id FROM products WHERE id = $1`, p1))
	if own == nil || *own != s1 {
		t.Errorf("SUP-11: an order through the rule supplier didn't make it the product's own (%v)", own)
	}
	if it := lowStock(t, s, s1)[p1]; it.SupplierWhy != "" {
		t.Errorf("SUP-12: own supplier now, still marked by rule %q", it.SupplierWhy)
	}
	s.Audit(t)
}

func TestSUP12_ProductAndSupplierPage(t *testing.T) {
	s, s1, p1 := ruleShop(t, "SUP12b")
	var d struct {
		SupplierID       int64
		SupplierName     string
		SupplierWhy      string
		RuleSupplierName string
	}
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/products/%d", p1), nil, &d))
	if d.SupplierID != 0 || d.RuleSupplierName != "SUP12b S1" {
		t.Errorf("SUP-12: product detail own %d rule %q", d.SupplierID, d.RuleSupplierName)
	}
	var l struct {
		Items []struct {
			ID           int64
			SupplierName string
			SupplierWhy  string
		}
	}
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/products?supplier=%d", s1), nil, &l))
	if len(l.Items) != 1 || l.Items[0].SupplierName != "SUP12b S1" || l.Items[0].SupplierWhy == "" {
		t.Errorf("SUP-12: products list %+v", l.Items)
	}
	var sup struct{ Products, ProductsByRule int }
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/suppliers/%d", s1), nil, &sup))
	if sup.Products != 0 || sup.ProductsByRule != 1 {
		t.Errorf("SUP-12: supplier page own %d by rule %d, want 0 and 1", sup.Products, sup.ProductsByRule)
	}
}

func TestSUP12_NeverClaimsLotsExport(t *testing.T) {
	s, s1, p1 := ruleShop(t, "SUP12c")
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p1, "quantity": "1", "reason": "damage", "note": "dropped"}, nil))
	var claims struct{ Waiting []struct{ ProductID int64 } }
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims", s1), nil, &claims))
	if slices.ContainsFunc(claims.Waiting, func(w struct{ ProductID int64 }) bool { return w.ProductID == p1 }) {
		t.Errorf("SUP-12: a loss went to the rule supplier's claims")
	}
	var lots struct{ Lots []struct{ SupplierID int64 } }
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/admin/stock/lots/%d", p1), nil, &lots))
	for _, l := range lots.Lots {
		if l.SupplierID != 0 {
			t.Errorf("SUP-12: a lot belongs to %d by rule", l.SupplierID)
		}
	}
	var sheet bytes.Buffer
	s.Must(t, s.Admin.Do("GET", "/api/v1/admin/products/export", nil, &sheet))
	recs, err := csv.NewReader(bytes.NewReader(sheet.Bytes())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	col := slices.Index(recs[0], "supplier")
	var mine []string
	for _, r := range recs[1:] {
		if r[0] == "SUP12c one" {
			mine = r
		}
	}
	if col < 0 || mine == nil || mine[col] != "" {
		t.Fatalf("SUP-12: export supplier column %d row %v", col, mine)
	}
	var back bytes.Buffer
	w := csv.NewWriter(&back)
	_ = w.WriteAll([][]string{recs[0], mine})
	s.Must(t, s.Admin.Upload("/api/v1/admin/products/import", "p.csv", back.Bytes(), nil))
	var own *int64
	s.Must(t, s.DB.Get(&own, `SELECT preferred_supplier_id FROM products WHERE id = $1`, p1))
	if own != nil {
		t.Errorf("SUP-11: a sheet round trip stamped supplier %d", *own)
	}
}

func TestSUP12_ReorderRhythm(t *testing.T) {
	s, s1, p1 := ruleShop(t, "SUP12d")
	other := s.Product(t, "SUP12d other", "20", "0", nil)
	for i := range 3 {
		var r struct{ PurchaseID int64 }
		s.Must(t, s.Admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": s1, "lines": []map[string]any{
			{"productId": other, "quantity": "5", "costPrice": "10"}}}, &r))
		s.DB.MustExec(`UPDATE stock_movements SET created_at = now() - make_interval(days => $1) WHERE reference_type = 'purchase' AND reference_id = $2`,
			7*(i+1), r.PurchaseID)
	}
	if err := reorder.RunIDs(context.Background(), s.DB, time.Now(), []int64{p1}); err != nil {
		t.Fatal(err)
	}
	var from string
	s.Must(t, s.DB.Get(&from, `SELECT COALESCE(reorder_calc->>'rhythmFrom', '') FROM products WHERE id = $1`, p1))
	if from != "supplier" {
		t.Errorf("SUP-12: reorder rhythm from %q, want the rule supplier's deliveries", from)
	}
}
