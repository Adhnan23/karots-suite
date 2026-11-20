package battle

import (
	"testing"
)

// Branches the main cases never reached (the coverage push, 2026-11-20).

// TEND-3: change given is never more than the change due — a cashier typing more
// hands back only what's due, and the drawer expects exactly that.
func TestTEND3_ChangeGivenNeverAboveDue(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	var r saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": []line{{"productId": s.prod["plain"], "quantity": "2"}}, "payments": []pay{cash("250")}, "changeGiven": "80"}, &r))
	if !dec(t, r.ChangeGiven).Equal(dec(t, "50")) {
		t.Errorf("TEND-3 change given %s, want 50 (Rs 250 for Rs 200; 80 typed)", r.ChangeGiven)
	}
	if got := s.expected(t, s.till1); got != "5200.00" {
		t.Errorf("TEND-3 drawer expects %s, want 5200.00", got)
	}
	s.audit(t)
}

// SALE-5 / STK-7: a picked lot with a blank (0) cost is charged at the product's
// cost, never booked as all profit.
func TestSALE5_ZeroCostLotSellsAtProductCost(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Torch", s.unitPcs, 0, nil) // product cost 10
	s.addLot(t, p, "5", "10", "")
	lot := s.lots(t, p)[0].ID
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE stock_batches SET cost_price = 0 WHERE product_id = (SELECT id FROM products WHERE name = 'Torch')`))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "2", "batchId": lot}}, cash("100"))
	var cost string
	s.must(t, s.db.Get(&cost, `SELECT cost_price FROM sale_items WHERE sale_id = $1`, r.ID))
	if !dec(t, cost).Equal(dec(t, "10")) {
		t.Errorf("SALE-5 a zero-cost lot's line costs %s, want the product's 10", cost)
	}
	if got := s.lotLeft(t, lot); got != "3.000" {
		t.Errorf("SALE-5 the picked lot has %s left, want 3.000", got)
	}
	s.audit(t)
}

// SALE-5 / SALE-7: a frozen product's picked lot only sets the price; a lot that
// has gone (merged, given back) sells at the product's price, never stops the sale.
func TestSALE7_FrozenGoneLotSellsAtProductPrice(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Pens", s.unitPcs, 0, nil) // sells at 20
	s.addLot(t, p, "5", "10", "")
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "3", "batchId": 999999999}}, cash("100"))
	if r.Total != "60.00" {
		t.Errorf("SALE-7 3 at the product's price: %s, want 60.00", r.Total)
	}
	var lot *int64
	s.must(t, s.db.Get(&lot, `SELECT batch_id FROM sale_items WHERE sale_id = $1`, r.ID))
	if lot != nil {
		t.Errorf("SALE-7 the line names lot %d, want none (the picked lot is gone)", *lot)
	}
	if got := s.onHand(t, p); got != "5.000" {
		t.Errorf("SALE-24 frozen stock moved to %s, want 5.000", got)
	}
	s.audit(t)
}

// RPT-22 / RPT-33: a warranty swap is a loss on the Losses page, at its movement's
// worth, and the page agrees with the P&L — no random day swaps a unit, so the
// agreement check never saw one.
func TestRPT33_WarrantySwapOnTheLossesPage(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph := s.prod["serial"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "10"}, nil))
	s.open(t, s.till1)
	s.must(t, s.swap(s.phoneSold(t, "RPT33-A", 0), "RPT33-B"))
	s.damage(t, s.prod["plain"], 0, "2")
	var worth string
	s.must(t, s.db.Get(&worth, `SELECT SUM(cost)::text FROM stock_movements WHERE type = 'warranty_replacement'`))
	var ls struct {
		WarrantyCount int
		WarrantyCost  string
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/losses", nil, &ls))
	if ls.WarrantyCount != 1 || !dec(t, ls.WarrantyCost).Equal(dec(t, worth)) || !dec(t, worth).IsPositive() {
		t.Errorf("RPT-22 Losses page: %d swaps worth %s, want 1 worth %s", ls.WarrantyCount, ls.WarrantyCost, worth)
	}
	s.agreeToday(t)
	s.audit(t)
}
