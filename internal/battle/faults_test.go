package battle

import (
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
