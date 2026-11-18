package battle

import (
	"fmt"
	"sync"
	"testing"
)

// SALE-50: the same sale sent twice (Pay pressed again after a lost answer, or
// two requests at once) is one sale: the second gets the first back.
func TestSALE50_SamePayTwiceIsOneSale(t *testing.T) {
	s := newShop(t)
	s.open(t, s.till1)
	body := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "clientRef": "till-battle-1",
		"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("100")}}
	var ids [4]int64
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var r saleResult
			if err := s.till1.Do("POST", "/api/v1/till/sales", body, &r); err != nil {
				t.Error(err)
			}
			ids[i] = r.ID
		}()
	}
	wg.Wait()
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM sales`))
	if n != 1 || ids[0] != ids[1] || ids[1] != ids[2] || ids[2] != ids[3] {
		t.Fatalf("%d sales for one Pay (ids %v)", n, ids)
	}
	s.audit(t)
}

// SALE-6 / SALE-23 / RET-1: two tills at once — selling the last units of a
// product, and returning the same sale — never take stock below zero or bring
// back more than was sold.
func TestTwoTillsAtOnce(t *testing.T) {
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["plain"], "newQuantity": "3", "note": "last three"}, nil))

	var wg sync.WaitGroup
	ok := make([]bool, 6)
	for i := range ok {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := s.till1
			if i%2 == 1 {
				c = s.till2
			}
			var r saleResult
			ok[i] = c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
				"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("100")}}, &r) == nil
		}()
	}
	wg.Wait()
	sold := 0
	for _, b := range ok {
		if b {
			sold++
		}
	}
	var onHand string
	s.must(t, s.db.Get(&onHand, `SELECT quantity FROM stock WHERE product_id = $1`, s.prod["plain"]))
	if sold != 3 || !dec(t, onHand).IsZero() {
		t.Errorf("sold %d of the last 3, on hand %s", sold, onHand)
	}

	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["odd"], "quantity": "2"}}, cash("100"))
	item := s.sale(t, s.till1, r.ID).Lines[0].ID
	backs := make([]bool, 4)
	for i := range backs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := s.till1
			if i%2 == 1 {
				c = s.till2
			}
			backs[i] = c.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
				"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, nil) == nil
		}()
	}
	wg.Wait()
	n := 0
	for _, b := range backs {
		if b {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d returns of 1 went through on a line of 2", n)
	}
	s.audit(t)
}
