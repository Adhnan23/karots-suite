package battle

import (
	"testing"
)

// SALE-2 / SALE-3: a wholesale sale rings each product's wholesale price, and its
// packs at their wholesale pack price; a combo's lines never take a pack price.
// (Gaps mutation testing found: nothing sold wholesale, or a pack in a combo.)
func TestSALE3_WholesaleAndPacksInCombos(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE products SET wholesale_price = 80 WHERE name = 'Battle plain'`))
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE product_packs SET wholesale_price = 120`))
	s.open(t, s.till1)
	ring := func(saleType string, items []line) string {
		var r saleResult
		s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": saleType, "discount": "0", "discountType": "fixed",
			"items": items, "payments": []pay{cash("10000")}}, &r))
		return r.Total
	}
	if got := ring("wholesale", []line{{"productId": s.prod["plain"], "quantity": "2"}}); got != "160.00" {
		t.Errorf("wholesale 2 × plain: %s, want 160.00 (Rs 80 each)", got)
	}
	if got := ring("retail", []line{{"productId": s.prod["plain"], "quantity": "2"}}); got != "200.00" {
		t.Errorf("retail 2 × plain: %s, want 200.00", got)
	}
	if got := ring("wholesale", []line{{"productId": s.prod["pack"], "quantity": "13"}}); got != "132.50" {
		t.Errorf("wholesale 13 of a Card of 12: %s, want 132.50 (one Card at 120 + 12.50)", got)
	}
	if got := ring("retail", []line{{"productId": s.prod["pack"], "quantity": "13"}}); got != "152.50" {
		t.Errorf("retail 13 of a Card of 12: %s, want 152.50 (one Card at 140 + 12.50)", got)
	}
	combo := map[string]any{"no": 1, "label": "Combo", "sets": "1"}
	if got := ring("retail", []line{{"productId": s.prod["pack"], "quantity": "12", "group": combo}, {"productId": s.prod["plain"], "quantity": "1", "group": combo}}); got != "250.00" {
		t.Errorf("12 in a combo: %s, want 250.00 (no pack price inside a combo)", got)
	}
	s.audit(t)
}

// RET-8: goods sold from a picked lot go back to that lot, which keeps the
// price it was sold at; any other line's return follows the product's price.
func TestRET8_ReturnedLotKeepsItsPrice(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	var lot int64
	s.must(t, s.db.Get(&lot, `SELECT id FROM stock_batches WHERE product_id = $1 ORDER BY id LIMIT 1`, s.prod["plain"]))
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE stock_batches SET selling_price = 90 WHERE product_id = (SELECT id FROM products WHERE name = 'Battle plain')`))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2", "batchId": lot}}, cash("1000"))
	if r.Total != "180.00" {
		t.Fatalf("2 from the Rs 90 lot: %s, want 180.00", r.Total)
	}
	s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "2")
	var price, left string
	s.must(t, s.db.QueryRow(`SELECT selling_price, qty_remaining FROM stock_batches WHERE id = $1`, lot).Scan(&price, &left))
	if !dec(t, price).Equal(dec(t, "90")) || !dec(t, left).Equal(dec(t, "100000")) {
		t.Errorf("the lot after the return: price %s, %s left; want 90 and all 100000 back", price, left)
	}
	s.audit(t)
}
