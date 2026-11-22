package battle

import (
	"sync"
	"sync/atomic"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// BAK-2: a shop at rest comes back row for row.
func TestBAK2_RoundTrip(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("200"))
	s.sale(t, s.till1, r.ID)
	ltest.RoundTrip(t, s.db)
}

// BAK-1: a backup taken while two tills sell is one moment — it restores and
// the rule book holds in it.
func TestBAK1_BackupWhileSelling(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	var stop atomic.Bool
	var wg sync.WaitGroup
	for _, c := range []*client{s.till1, s.till2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				var r saleResult
				if err := c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0",
					"discountType": "fixed", "items": []line{{"productId": s.prod["plain"], "quantity": "1"},
						{"productId": s.prod["odd"], "quantity": "3"}}, "payments": []pay{cash("200")}}, &r); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for range 8 {
		ltest.Restored(t, s.db)
	}
	stop.Store(true)
	wg.Wait()
}
