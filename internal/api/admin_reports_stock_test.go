package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Valuation: the product rows add up to the headline (both by lots), across
// pages. Count sheet: a parent category pulls in its sub-categories. Top-up:
// the gap is available − need. Reorder ?all=1: past the 100-row page cap.
func TestStockReports(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var unit, parent, child int64
	te.db.Get(&unit, `SELECT id FROM units LIMIT 1`)                                                            //nolint:errcheck
	te.db.Get(&parent, `INSERT INTO categories (name) VALUES ('T rs Pantry') RETURNING id`)                     //nolint:errcheck
	te.db.Get(&child, `INSERT INTO categories (name, parent_id) VALUES ('T rs Rice', $1) RETURNING id`, parent) //nolint:errcheck
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM products WHERE name LIKE 'T rs %'`)   //nolint:errcheck
		te.db.Exec(`DELETE FROM categories WHERE name LIKE 'T rs %'`) //nolint:errcheck
	})
	// 104 rice packs under the child (2 on hand, one lot of 1 bought at 7, the
	// product now costs 10; reorder at 5) and one jar on the parent itself.
	for i := 0; i < 104; i++ {
		var id int64
		te.db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, reorder_level)
			VALUES ($1,$2,$3,10,15,5) RETURNING id`, fmt.Sprintf("T rs rice %03d", i), child, unit) //nolint:errcheck
		te.db.MustExec(`INSERT INTO stock (product_id, quantity) VALUES ($1, 2) ON CONFLICT (product_id) DO UPDATE SET quantity = 2`, id)
		te.db.MustExec(`INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, source) VALUES ($1, 1, 1, 7, 'opening')`, id)
	}
	var jar int64
	te.db.Get(&jar, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('T rs jar', $1, $2, 100, 150) RETURNING id`, parent, unit) //nolint:errcheck
	te.db.MustExec(`INSERT INTO stock (product_id, quantity) VALUES ($1, 3) ON CONFLICT (product_id) DO UPDATE SET quantity = 3`, jar)

	q := fmt.Sprintf("category=%d", parent)
	inv := decode[InventoryReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/inventory?"+q, nil, admin))
	// Each pack: 7 (its lot) + 1 × 10 (stock no lot covers) = 17; the jar 300.
	if inv.CostValue != "2068.00" || inv.SKUs != 105 || len(inv.Breadcrumb) != 1 || len(inv.Children) != 2 {
		t.Fatalf("inventory %+v", inv)
	}
	sum, n := decimal.Zero, 0
	for off := 0; ; off += counterPage {
		rows := decode[[]InventoryProduct](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reports/inventory/products?%s&offset=%d", q, off), nil, admin))
		for _, r := range rows {
			sum = sum.Add(decimal.RequireFromString(r.CostValue))
		}
		n += len(rows)
		if len(rows) < counterPage {
			break
		}
	}
	if n != 105 || sum.StringFixed(2) != inv.CostValue {
		t.Fatalf("rows %d sum %s, headline %s", n, sum, inv.CostValue)
	}

	cs := decode[CountSheet](t, te.do(http.MethodGet, "/api/v1/admin/reports/count-sheet?"+q, nil, admin))
	if len(cs.Rows) != 105 || cs.Category != "T rs Pantry" || cs.Rows[0].Category != "T rs Pantry" || cs.Rows[1].Category != "T rs Pantry > T rs Rice" {
		t.Fatalf("count sheet %d %q %+v", len(cs.Rows), cs.Category, cs.Rows[:2])
	}

	tp := decode[TopupReport](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reports/topup?category=%d", parent), nil, admin))
	avail, need := decimal.RequireFromString(tp.Available), decimal.RequireFromString(tp.TotalNeed)
	if len(tp.Stock) != 104 || tp.StockNeed != "3120.00" || tp.Gap != avail.Sub(need).StringFixed(2) {
		t.Fatalf("top-up %d need %s gap %s", len(tp.Stock), tp.StockNeed, tp.Gap)
	}

	ro := decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?all=1&category=%d", child), nil, admin))
	if len(ro.Items) != 104 || ro.Total != 104 {
		t.Fatalf("reorder all %d of %d", len(ro.Items), ro.Total)
	}
}
