package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Smart reorder levels on the admin side: the learned numbers drive the
// suggestion, derived products never show, Fixed levels that look off are
// listed and fixed in one tap, and a category switches to Auto.
func TestAdminReorderSmart(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Smart %d", time.Now().UnixNano())}, ck))
	// scarce: Fixed at 5, 1 on hand; sales say 2 a day, supplier every 7 days → 20.
	te.db.Exec(`UPDATE products SET reorder_auto = false, reorder_level = 5, preferred_supplier_id = $1,
		reorder_calc = '{"daily":2,"rhythmDays":7,"rhythmFrom":"product","regular":true,"safetyDays":3,"calcLevel":20,"learnedPct":100}'
		WHERE id = $2`, sup.ID, f.scarceID) //nolint:errcheck
	list := func(q string) ReorderList {
		return decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d%s", sup.ID, q), nil, ck))
	}

	l := list("")
	if l.Total != 1 {
		t.Fatalf("want scarce only: %+v", l)
	}
	r := l.Items[0]
	// level 5 + 2 a day × 7 days − 1 on hand = 18
	if r.Suggested != "18" || r.Auto || r.Calc == nil || r.Calc.CalcLevel != 20 || !r.LooksOff {
		t.Fatalf("row: %+v calc %+v", r, r.Calc)
	}

	// Made from another product (loose from a bag) → never on the list.
	var conv int64
	if err := te.db.Get(&conv, `INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 10) RETURNING id`, f.plainID, f.scarceID); err != nil {
		t.Fatal(err)
	}
	if l := list(""); l.Total != 0 {
		t.Fatalf("derived product listed: %+v", l)
	}
	te.db.Exec(`DELETE FROM product_conversions WHERE id = $1`, conv) //nolint:errcheck

	// Levels look off: listed even when not low.
	te.db.Exec(`UPDATE stock SET quantity = 500 WHERE product_id = $1`, f.scarceID) //nolint:errcheck
	if l := list("&looksOff=1"); l.Total != 1 || l.Items[0].ProductID != f.scarceID {
		t.Fatalf("looks off: %+v", l)
	}

	// Use 20.
	if rec := te.do(http.MethodPost, "/api/v1/admin/products/reorder-level", ReorderLevelRequest{IDs: []int64{f.scarceID}, Use: "suggested"}, ck); rec.Code != http.StatusOK {
		t.Fatalf("use: %d %s", rec.Code, rec.Body)
	}
	var level int
	var auto bool
	te.db.QueryRow(`SELECT reorder_level, reorder_auto FROM products WHERE id = $1`, f.scarceID).Scan(&level, &auto) //nolint:errcheck
	if level != 20 || auto {
		t.Fatalf("after Use: level %d auto %v, want 20 Fixed", level, auto)
	}
	if l := list("&looksOff=1"); l.Total != 0 {
		t.Fatalf("still looks off after Use: %+v", l)
	}

	// Switch the whole category to Auto: starts from the current number.
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id = $1`, f.scarceID) //nolint:errcheck
	te.db.Exec(`UPDATE products SET reorder_auto = false WHERE category_id = $1`, cat) //nolint:errcheck
	res := decode[ReorderAutoResult](t, te.do(http.MethodPost, "/api/v1/admin/products/reorder-auto", ReorderAutoRequest{CategoryID: cat}, ck))
	if res.Switched < 4 {
		t.Fatalf("switched %d, want the fixture's 4+", res.Switched)
	}
	var start int
	te.db.QueryRow(`SELECT reorder_auto, reorder_start FROM products WHERE id = $1`, f.scarceID).Scan(&auto, &start) //nolint:errcheck
	if !auto || start != 20 {
		t.Fatalf("after switch: auto %v start %d, want true 20", auto, start)
	}

	// The product form shows the mode and the learned numbers.
	p := decode[AdminProduct](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/products/%d", f.scarceID), nil, ck))
	if !p.ReorderAuto || p.ReorderStart != 20 || p.ReorderCalc == nil {
		t.Fatalf("product: auto %v start %d calc %v", p.ReorderAuto, p.ReorderStart, p.ReorderCalc)
	}
}
