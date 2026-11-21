// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own).
package battle

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/combos"
)

// Combos' battle tests (docs/rules/plugins/combos.md): one throwaway shop on the
// real server with this plugin compiled in (ltest.Main).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

// pair makes two stocked products (Rs 100 and Rs 50) and a 10 % off combo of one each.
func pair(t *testing.T, s *ltest.Shop, name string) (combo, a, b int64) {
	t.Helper()
	n := seq.Add(1)
	a = s.Product(t, fmt.Sprintf("%s A %d", name, n), "100", "50", nil)
	b = s.Product(t, fmt.Sprintf("%s B %d", name, n), "50", "50", nil)
	return create(t, s, name, "percent", "10", a, b), a, b
}

func create(t *testing.T, s *ltest.Shop, name, typ, value string, ids ...int64) int64 {
	t.Helper()
	var c combos.ComboDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/combos", body(name, typ, value, ids...), &c))
	return c.ID
}

func body(name, typ, value string, ids ...int64) map[string]any {
	items := []map[string]any{}
	for _, id := range ids {
		items = append(items, map[string]any{"productId": id, "qty": "1"})
	}
	return map[string]any{"name": name, "discountType": typ, "discountValue": value, "items": items}
}

// comboLines are a's and b's lines of `sets` sets of a 10 % pair, as the till rings
// them: one group, each line its per-unit discount, tagged with the combo.
func comboLines(combo, a, b int64, group int, sets string) []map[string]any {
	g := map[string]any{"no": group, "label": "Deal (combo)", "sets": sets}
	tag := map[string]any{"combos": map[string]any{"comboId": combo}}
	return []map[string]any{
		{"productId": a, "quantity": sets, "discount": "10", "discountType": "fixed", "group": g, "plugin": tag},
		{"productId": b, "quantity": sets, "discount": "5", "discountType": "fixed", "group": g, "plugin": tag},
	}
}

type sold struct {
	ID    int64
	Total string
	Lines []struct{ ID int64 }
}

func sell(t *testing.T, s *ltest.Shop, items []map[string]any) sold {
	t.Helper()
	var r sold
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": items, "payments": []map[string]string{{"method": "cash", "amount": "10000"}}}, &r))
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &r))
	return r
}

func row(t *testing.T, s *ltest.Shop, saleID, combo int64) (sets, net, discount string, ok bool) {
	t.Helper()
	err := s.DB.QueryRow(`SELECT sets::text, net::text, discount::text FROM combo_sales WHERE sale_id = $1 AND combo_id = $2`, saleID, combo).
		Scan(&sets, &net, &discount)
	return sets, net, discount, err == nil
}

func report(t *testing.T, s *ltest.Shop, combo int64) combos.ReportRowDTO {
	t.Helper()
	var r combos.ReportDTO
	day := time.Now().Format("2006-01-02")
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/combos/report?from="+day+"&to="+day, nil, &r))
	for _, x := range r.Rows {
		if x.ComboID != nil && *x.ComboID == combo {
			return x
		}
	}
	return combos.ReportRowDTO{}
}

var tillOnce atomic.Bool

func openTill(t *testing.T, s *ltest.Shop) {
	if tillOnce.CompareAndSwap(false, true) {
		s.OpenTill(t, s.Cashier)
	}
}

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c, a, b := pair(t, s, "Fire")
	sale := sell(t, s, comboLines(c, a, b, 1, "2"))
	dup := s.Product(t, "Fire dup", "10", "0", nil)
	for _, x := range []struct{ rule, bad string }{
		{"CMB-10", fmt.Sprintf(`UPDATE combos SET discount_type = 'free' WHERE id = %d`, c)},
		{"CMB-10", fmt.Sprintf(`UPDATE combos SET discount_value = 101 WHERE id = %d`, c)},
		{"CMB-10", fmt.Sprintf(`UPDATE combos SET starts_on = '2026-02-01', ends_on = '2026-01-01' WHERE id = %d`, c)},
		{"CMB-10", fmt.Sprintf(`UPDATE combo_items SET qty = 0 WHERE combo_id = %d AND product_id = %d`, c, a)},
		{"CMB-10", fmt.Sprintf(`UPDATE combo_sales SET sets = 0 WHERE sale_id = %d`, sale.ID)},
		{"CMB-5", fmt.Sprintf(`UPDATE combo_sales SET net = net + 1 WHERE sale_id = %d`, sale.ID)},
		{"CMB-5", fmt.Sprintf(`UPDATE combo_sales SET discount = discount + 1 WHERE sale_id = %d`, sale.ID)},
		{"CMB-5", fmt.Sprintf(`UPDATE combo_sales SET sets = 3 WHERE sale_id = %d`, sale.ID)},
		{"CMB-5", fmt.Sprintf(`UPDATE combo_sales SET item_ids = item_ids || 999999::bigint WHERE sale_id = %d`, sale.ID)},
		{"CMB-9", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, dup, a)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(x.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == x.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", x.rule, x.bad)
		}
	}
}

// CMB-4, CMB-5: a combo sale is an ordinary sale and its sold row is written with
// it, from its own lines (no after-sale call).
func TestCMB5_SoldWithTheSale(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c, a, b := pair(t, s, "With sale")
	// Two groups of the same combo on one bill: 2 sets + 1 set.
	sale := sell(t, s, append(comboLines(c, a, b, 1, "2"), comboLines(c, a, b, 2, "1")...))
	if sale.Total != "405.00" {
		t.Errorf("CMB-4: total %s, want 405.00 (3 sets of 150 less 10 %%)", sale.Total)
	}
	sets, net, off, ok := row(t, s, sale.ID, c)
	if !ok {
		t.Fatal("CMB-5: no sold row written with the sale")
	}
	if sets != "3.0000" || net != "405.00" || off != "45.00" {
		t.Errorf("CMB-5: sold row %s sets, net %s, discount %s; want 3, 405.00, 45.00", sets, net, off)
	}
	// A plain sale of the same products writes none.
	plain := sell(t, s, []map[string]any{{"productId": a, "quantity": "1"}, {"productId": b, "quantity": "1"}})
	if _, _, _, ok := row(t, s, plain.ID, c); ok {
		t.Error("CMB-5: a plain sale got a combo row")
	}
	// A tag naming a combo that doesn't exist is skipped, never refused (a sale is never stopped).
	ghost := sell(t, s, comboLines(999999, a, b, 1, "1"))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM combo_sales WHERE sale_id = $1`, ghost.ID))
	if n != 0 {
		t.Errorf("CMB-5: %d rows for a combo that doesn't exist", n)
	}
	s.Audit(t)
}

// CMB-6: a combo comes back whole, in whole sets, and the report takes them off.
func TestCMB6_ReturnsOffTheReport(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c, a, b := pair(t, s, "Returned")
	sale := sell(t, s, comboLines(c, a, b, 1, "2"))
	if r := report(t, s, c); r.Sets != "2" || r.Net != "270.00" || r.Discount != "30.00" {
		t.Fatalf("CMB-6: before the return %+v", r)
	}
	// One line alone is refused (RET-13).
	if err := s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", sale.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": sale.Lines[0].ID, "quantity": "1"}}}, nil); err == nil {
		t.Error("RET-13: one line of a combo came back alone")
	}
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", sale.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": sale.Lines[0].ID, "quantity": "1"}, {"saleItemId": sale.Lines[1].ID, "quantity": "1"}}}, nil))
	if r := report(t, s, c); r.Sets != "1" || r.Net != "135.00" || r.Discount != "15.00" {
		t.Errorf("CMB-6: after one set came back the report shows %s sets, net %s, discount %s; want 1, 135.00, 15.00", r.Sets, r.Net, r.Discount)
	}
	var list []combos.ComboDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/combos", nil, &list))
	for _, x := range list {
		if x.ID == c && (x.SoldSets != "1" || x.SoldNet != "135.00") {
			t.Errorf("CMB-6: the combo's sold count %s / %s, want 1 / 135.00", x.SoldSets, x.SoldNet)
		}
	}
	s.Audit(t)
}

// CMB-2: the till sees exactly the on combos; a disabled member says why.
func TestCMB2_TillSeesOnCombos(t *testing.T) {
	s := ltest.Shared(t).Shop
	on, a, _ := pair(t, s, "On")
	off, _, _ := pair(t, s, "Off")
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/combos/%d/active", off), map[string]bool{"active": false}, nil))
	n := seq.Add(1)
	la := s.Product(t, fmt.Sprintf("Later A %d", n), "100", "5", nil)
	lb := s.Product(t, fmt.Sprintf("Later B %d", n), "50", "5", nil)
	bl := body("Later", "percent", "10", la, lb)
	bl["startsOn"] = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	var got combos.ComboDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/combos", bl, &got))
	later := got.ID
	var till []combos.ComboDTO
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/combos/till", nil, &till))
	seen := map[int64]bool{}
	for _, x := range till {
		seen[x.ID] = true
	}
	if !seen[on] || seen[off] || seen[later] {
		t.Errorf("CMB-2: till shows on=%v off=%v scheduled=%v", seen[on], seen[off], seen[later])
	}
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", a), nil, nil))
	if p := find(t, s, on).Problem; p == "" {
		t.Error("CMB-2: a combo with a disabled member shows no problem")
	}
}

func find(t *testing.T, s *ltest.Shop, id int64) combos.ComboDTO {
	t.Helper()
	var list []combos.ComboDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/combos", nil, &list))
	for _, x := range list {
		if x.ID == id {
			return x
		}
	}
	t.Fatalf("combo %d not listed", id)
	return combos.ComboDTO{}
}

// CMB-8: deleting a combo keeps what it sold.
func TestCMB8_DeleteKeepsHistory(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c, a, b := pair(t, s, "Gone")
	sell(t, s, comboLines(c, a, b, 1, "1"))
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/combos/%d", c), nil, nil))
	var r combos.ReportDTO
	day := time.Now().Format("2006-01-02")
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/combos/report?from="+day+"&to="+day, nil, &r))
	found := false
	for _, x := range r.Rows {
		found = found || (x.ComboID == nil && x.Name == "Gone" && x.Sets == "1")
	}
	if !found {
		t.Errorf("CMB-8: the deleted combo's sale left the report: %+v", r.Rows)
	}
	s.Audit(t)
}

// CMB-9: a merge moves the duplicate's member line to the kept product.
func TestCMB9_ProductMerge(t *testing.T) {
	s := ltest.Shared(t).Shop
	c, a, _ := pair(t, s, "Merge")
	kept := s.Product(t, "Merge kept", "100", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", a, kept), map[string]string{"lotPrice": "follow"}, nil))
	if m := find(t, s, c).Members; m[0].ProductID != kept {
		t.Errorf("CMB-9: member is %d, want the kept %d", m[0].ProductID, kept)
	}
	s.Audit(t)
}

// CMB-5: a combo of one product ×2 rings as a single line and still gets its row;
// the till's list never sends warnings as null.
func TestCMB5_OneLineCombo(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	p := s.Product(t, fmt.Sprintf("Twin %d", seq.Add(1)), "100", "10", nil)
	var c combos.ComboDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/combos", map[string]any{"name": "Two for 180", "discountType": "price", "discountValue": "180",
		"items": []map[string]any{{"productId": p, "qty": "2"}}}, &c))
	if c.Warnings == nil {
		t.Error("CMB-2: warnings sent as null")
	}
	sale := sell(t, s, []map[string]any{{"productId": p, "quantity": "2", "discount": "10", "discountType": "fixed",
		"group": map[string]any{"no": 1, "label": "Two for 180 (combo)", "sets": "1"}, "plugin": map[string]any{"combos": map[string]any{"comboId": c.ID}}}})
	if sets, net, _, ok := row(t, s, sale.ID, c.ID); !ok || sets != "1.0000" || net != "180.00" {
		t.Errorf("CMB-5: one-line combo row %v: %s sets, net %s; want 1, 180.00", ok, sets, net)
	}
	var v struct{ AutoApply bool }
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/combos/settings", map[string]bool{"autoApply": true}, &v))
	s.Must(t, s.Admin.Do("PUT", "/api/v1/x/combos/settings", map[string]bool{"autoApply": false}, nil))
	if !v.AutoApply {
		t.Error("CMB-7: saving settings doesn't answer what was saved")
	}
	s.Audit(t)
}
