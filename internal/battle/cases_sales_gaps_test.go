package battle

import (
	"fmt"
	"testing"
)

// Sales branches no case reached until the mutant replay (coverage push, 2026-11-21).

// SALE-5 / SALE-7: a frozen product's picked lot that is still there sets the
// line's price (only a gone lot falls back to the product's), and stock doesn't move.
func TestSALE7_FrozenPickedLotSetsThePrice(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Frozen ink", s.unitPcs, 0, nil) // sells at 20
	s.addLot(t, p, "5", "10", "")
	lot := s.lots(t, p)[0].ID
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/price", lot), map[string]string{"sellingPrice": "25"}, nil))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "2", "batchId": lot}}, cash("100"))
	if r.Total != "50.00" {
		t.Errorf("SALE-5 2 from the Rs 25 lot of a frozen product: %s, want 50.00", r.Total)
	}
	if got := s.onHand(t, p); got != "5.000" {
		t.Errorf("SALE-24 frozen stock moved to %s, want 5.000", got)
	}
	s.audit(t)
}

// RET-8: only a line rung from a picked lot carries its price back to the lot; a
// wholesale line sold oldest-first goes back without stamping the wholesale price.
func TestRET8_OtherLinesFollowTheProduct(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Wholesale soap", s.unitPcs, 0, map[string]any{"wholesalePrice": "15"}) // retail 20
	s.addLot(t, p, "10", "10", "")
	s.open(t, s.till1)
	r := s.sellWith(t, map[string]any{"saleType": "wholesale"}, []line{{"productId": p, "quantity": "2"}})
	s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "2")
	var price *string // the goods come back as a new lot
	s.must(t, s.db.Get(&price, `SELECT selling_price::text FROM stock_batches WHERE product_id = $1 ORDER BY id DESC LIMIT 1`, p))
	if price != nil && !dec(t, *price).IsZero() {
		t.Errorf("RET-8 a wholesale line's return stamped Rs %s on the lot, want it to follow the product", *price)
	}
	s.audit(t)
}

// SALE-9 / SALE-25: a service sold with no components of its own uses its recipe —
// a cup of tea takes 0.25 of a packet — without any plugin in the way.
func TestSALE25_RecipeComponentsMove(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	tea := s.stocked(t, "Tea packet", s.unitKg, 0, nil) // cost 10, a decimal unit
	s.addLot(t, tea, "4", "10", "")
	var made struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/services", map[string]any{"name": "Cup of tea", "categoryId": s.cat, "unitId": s.unitPcs, "sellingPrice": "50"}, &made))
	cup := made.ID
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/recipe", cup), map[string]any{
		"components": []map[string]any{{"productId": tea, "amount": "0.25", "mode": "per_unit"}}}, nil))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}, {"productId": cup, "quantity": "2"}}, cash("1000"))
	if got := s.onHand(t, tea); got != "3.500" {
		t.Errorf("SALE-25 2 cups took the tea to %s, want 3.500", got)
	}
	var cost string
	s.must(t, s.db.Get(&cost, `SELECT cost_price::text FROM sale_items WHERE sale_id = $1 AND product_id = $2`, r.ID, cup))
	if !dec(t, cost).Equal(dec(t, "2.5")) {
		t.Errorf("SALE-9 a cup costs %s, want its 0.25 kg of Rs 10 tea = 2.50", cost)
	}
	s.audit(t)
}

// SALE-15: prepared-by names an existing person or nobody, and never costs the sale.
func TestSALE15_PreparedByKnownOrNobody(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	known := s.userID(t, "0770000001")
	for _, c := range []struct {
		by   int64
		want *int64
	}{{known, &known}, {999999, nil}} {
		r := s.sellWith(t, map[string]any{"preparedBy": c.by}, []line{{"productId": s.prod["plain"], "quantity": "1"}})
		var got *int64
		s.must(t, s.db.Get(&got, `SELECT prepared_by FROM sales WHERE id = $1`, r.ID))
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("SALE-15 prepared by %d: stored %v, want %v", c.by, got, c.want)
		}
	}
	s.audit(t)
}

// RET-3: the refund slip shows what the return refunded (a reprint reads it back).
func TestRET3_RefundSlipShowsTheRefund(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "3"}}, cash("500"))
	refund := s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "2")
	var slip struct{ Refund struct{ Refund string } }
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/refund-print?data=1", r.ID), nil, &slip))
	if !dec(t, slip.Refund.Refund).Equal(refund) {
		t.Errorf("RET-3 the refund slip says %s, the return refunded %s", slip.Refund.Refund, refund)
	}
}

// sellWith rings a cash sale on till 1 with extra fields (sale type, prepared by),
// paid in exact cash.
func (s *shop) sellWith(t *testing.T, extra map[string]any, items []line) saleResult {
	t.Helper()
	body := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "items": items, "payments": []pay{cash("100000")}}
	for k, v := range extra {
		body[k] = v
	}
	var r saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", body, &r))
	return r
}

// SALE-5: a lot's own price holds until the product's new price is applied "to
// the stock I already have" — then the lot sells at the new price.
func TestSALE5_RepriceStockClearsLotPrices(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Repriced tea", s.unitPcs, 0, nil) // sells at 20
	s.addLot(t, p, "5", "10", "")
	lot := s.lots(t, p)[0].ID
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/batches/%d/price", lot), map[string]string{"sellingPrice": "25"}, nil))
	put := func(reprice bool) {
		s.must(t, s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", p), map[string]any{"name": "Repriced tea", "categoryId": s.cat,
			"unitId": s.unitPcs, "costPrice": "10", "sellingPrice": "30", "wholesalePrice": "30", "reorderAuto": false, "repriceStock": reprice}, nil))
	}
	s.open(t, s.till1)
	put(false)
	if r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "1", "batchId": lot}}, cash("100")); r.Total != "25.00" {
		t.Errorf("SALE-5 the lot's own price after a plain price change: %s, want 25.00", r.Total)
	}
	put(true)
	if r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "1", "batchId": lot}}, cash("100")); r.Total != "30.00" {
		t.Errorf("SALE-5 after the new price is applied to stock on hand: %s, want 30.00", r.Total)
	}
	s.audit(t)
}
