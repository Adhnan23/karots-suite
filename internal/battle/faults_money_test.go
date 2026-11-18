package battle

import (
	"sync"
	"testing"
)

// DRAW-16: cash-outs from one drawer at the same moment (two windows) can't
// together take more than it holds: of eight Rs 1,000 withdrawals from a
// Rs 5,000 drawer, five go through.
func TestDRAW16_WithdrawalsAtOnceNeverOverdraw(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "0")
	s.open(t, s.till1)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "1000", "reason": "x", "lockerId": safe}, nil) == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if got := s.expected(t, s.till1); ok != 5 || got != "0.00" {
		t.Errorf("%d withdrawals went through, drawer expects %s (want 5, 0.00)", ok, got)
	}
	s.audit(t)
}
