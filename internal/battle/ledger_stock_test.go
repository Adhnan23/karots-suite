package battle

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
)

// lots is a product's live lots, FEFO order (Batches).
func (s *shop) lots(t testing.TB, id int64) []struct {
	ID           int64  `json:"id"`
	QtyRemaining string `json:"qtyRemaining"`
	CostPrice    string `json:"costPrice"`
	ExpiryDate   string `json:"expiryDate"`
} {
	var r struct {
		Lots []struct {
			ID           int64  `json:"id"`
			QtyRemaining string `json:"qtyRemaining"`
			CostPrice    string `json:"costPrice"`
			ExpiryDate   string `json:"expiryDate"`
		} `json:"lots"`
	}
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/stock/lots/%d", id), nil, &r))
	return r.Lots
}

// stocked makes a product in its own unit with a preferred supplier and no stock.
func (s *shop) stocked(t testing.TB, name string, unit, supplier int64, extra map[string]any) int64 {
	body := map[string]any{"name": name, "categoryId": s.cat, "unitId": unit, "costPrice": "10", "sellingPrice": "20",
		"wholesalePrice": "20", "reorderAuto": false}
	if supplier > 0 {
		body["supplierId"] = supplier
	}
	for k, v := range extra {
		body[k] = v
	}
	var p struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/products", body, &p))
	return p.ID
}

func (s *shop) addLot(t testing.TB, id int64, qty, cost, expiry string) {
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/add", id),
		map[string]string{"qty": qty, "cost": cost, "expiry": expiry}, nil))
}

// A clean day of every area-4 action breaks no rule; each corruption below breaks
// the rule it names (the auditor sees what it is meant to see).
func TestStockRulesSeeCorruption(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Battle Supplier", "phone": "0112000001"}, &sup))
	pills := s.stocked(t, "Pills", s.unitPcs, sup.ID, map[string]any{"packs": []map[string]string{{"name": "Card", "size": "10", "sellingPrice": "180"}}})
	bag := s.stocked(t, "Rice bag", s.unitPcs, 0, nil)
	rice := s.stocked(t, "Rice loose", s.unitKg, 0, nil)
	shelf := s.stocked(t, "Pencils", s.unitPcs, 0, nil)

	// Lots in: two dated lots and an undated one; a top-up of a named lot.
	s.addLot(t, pills, "40", "8", "2027-01-31")
	s.addLot(t, pills, "30", "9", "2027-06-30")
	s.addLot(t, pills, "20", "", "")
	l := s.lots(t, pills)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": pills, "newQuantity": "95", "batchId": l[1].ID}, nil))
	// Counts: the whole product down, then one lot.
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": pills, "newQuantity": "93", "note": "count"}, nil))
	l = s.lots(t, pills)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": pills, "newQuantity": "92", "batchId": l[0].ID}, nil))
	// Sales: FEFO, a picked lot, a card, an oversell of the bag.
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": pills, "quantity": "3"}}, cash("60"))
	sold := s.sell(t, s.till1, "0", []line{{"productId": pills, "quantity": "2", "batchId": l[1].ID}}, cash("40"))
	s.sell(t, s.till1, "0", []line{{"productId": pills, "quantity": "10"}}, cash("180"))
	s.addLot(t, bag, "2", "2500", "")
	s.sell(t, s.till1, "0", []line{{"productId": bag, "quantity": "3", "allowOversell": true}}, cash("60"))
	s.giveBack(t, s.till1, sold.ID, s.sale(t, s.till1, sold.ID).Lines[0].ID, "1")
	// Write-offs: damage from a named lot, own use and staff FEFO.
	for _, r := range []map[string]any{{"reason": "damage", "batchId": l[2].ID}, {"reason": "own_use"}, {"reason": "staff"}} {
		r["productId"], r["quantity"] = pills, "1"
		s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", r, nil))
	}
	// Recall the first lot, then give it back counting one more than the books.
	l = s.lots(t, pills)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup.ID, l[0].ID), map[string]bool{"stopSelling": true}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/give-back", sup.ID, l[0].ID),
		map[string]string{"counted": dec(t, l[0].QtyRemaining).Add(dec(t, "1")).String()}, nil))
	// Two lots merged.
	l = s.lots(t, pills)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/lots/%d/merge", pills), map[string]any{"batchIds": []int64{l[0].ID, l[1].ID}}, nil))
	// A bag opened: 25 kg asked, 24.6 weighed out.
	s.addLot(t, bag, "3", "2500", "2027-03-01")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": bag, "toProductId": rice, "ratio": "25"}, &cv))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "1", "toQty": "24.6"}, nil))
	// Freeze: 200 counted in storage, sales don't touch it, a box out, unfreeze with 13 out.
	s.addLot(t, shelf, "200", "5", "")
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{shelf}}, nil))
	s.sell(t, s.till1, "0", []line{{"productId": shelf, "quantity": "5"}}, cash("100"))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", shelf), map[string]string{"quantity": "10"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": shelf, "quantity": "1", "reason": "damage", "fromShelf": true}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/unfreeze", shelf), map[string]string{"shelfQty": "13"}, nil))
	s.audit(t)

	cases := []struct{ rule, breakIt, mend string }{
		{"STK-2", `UPDATE stock_batches SET qty_remaining = qty_remaining + 1 WHERE id = (SELECT max(id) FROM stock_batches WHERE product_id = PILLS)`,
			`UPDATE stock_batches SET qty_remaining = qty_remaining - 1 WHERE id = (SELECT max(id) FROM stock_batches WHERE product_id = PILLS)`},
		{"STK-3", `UPDATE stock SET quantity = quantity - 3 WHERE product_id = BAG; INSERT INTO stock_movements (product_id, type, quantity, user_id, note) VALUES (BAG, 'adjust', -3, 1, 'neg')`,
			`UPDATE stock SET quantity = quantity + 3 WHERE product_id = BAG; DELETE FROM stock_movements WHERE note = 'neg'`},
		{"STK-7", `UPDATE stock_movements SET cost = -cost WHERE id = (SELECT max(id) FROM stock_movements WHERE type = 'staff')`,
			`UPDATE stock_movements SET cost = -cost WHERE id = (SELECT max(id) FROM stock_movements WHERE type = 'staff')`},
		{"STK-7", `UPDATE stock_movements SET note = cost::text, cost = 0 WHERE id = (SELECT max(id) FROM stock_movements WHERE type = 'own_use')`,
			`UPDATE stock_movements SET cost = note::numeric, note = NULL WHERE id = (SELECT max(id) FROM stock_movements WHERE type = 'own_use')`},
		{"STK-17", `UPDATE product_packs SET size = 2.5 WHERE product_id = PILLS`, `UPDATE product_packs SET size = 10 WHERE product_id = PILLS`},
		{"STK-18", `UPDATE stock SET quantity = quantity + 0.5 WHERE product_id = PILLS; UPDATE stock_batches SET qty_remaining = qty_remaining + 0.5 WHERE id = (SELECT max(id) FROM stock_batches WHERE product_id = PILLS); INSERT INTO stock_movements (product_id, type, quantity, user_id, note) VALUES (PILLS, 'adjust', 0.5, 1, 'half')`,
			`UPDATE stock SET quantity = quantity - 0.5 WHERE product_id = PILLS; UPDATE stock_batches SET qty_remaining = qty_remaining - 0.5 WHERE id = (SELECT max(id) FROM stock_batches WHERE product_id = PILLS); DELETE FROM stock_movements WHERE note = 'half'`},
	}
	ids := strings.NewReplacer("PILLS", fmt.Sprint(pills), "SHELF", fmt.Sprint(shelf), "BAG", fmt.Sprint(bag))
	for _, c := range cases {
		if _, err := s.db.ExecContext(ctx, ids.Replace(c.breakIt)); err != nil {
			t.Fatalf("%s: %v", c.rule, err)
		}
		vs, err := ledgercheck.Run(ctx, s.db)
		if _, merr := s.db.ExecContext(ctx, ids.Replace(c.mend)); merr != nil {
			t.Fatalf("%s mend: %v", c.rule, merr)
		}
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, v := range vs {
			got = append(got, v.Rule)
		}
		if !slices.Contains(got, c.rule) {
			t.Errorf("%s: corruption not seen (got %v)", c.rule, got)
		}
	}
	s.audit(t)
}
