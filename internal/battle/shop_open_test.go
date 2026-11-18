package battle

import "testing"

// The harness itself: a shop opens a till, sells, closes, and keeps every rule.
func TestShopOpens(t *testing.T) {
	s := newShop(t)
	s.open(t, s.till1)
	var sale saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{
		"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items":    []map[string]any{{"productId": s.prod["plain"], "quantity": "2"}},
		"payments": []map[string]any{{"method": "cash", "amount": "500"}},
	}, &sale))
	if sale.Total != "200.00" {
		t.Fatalf("total %s, want 200.00", sale.Total)
	}
	s.close(t, s.till1)
	s.audit(t)
}
