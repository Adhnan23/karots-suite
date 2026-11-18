package battle

import (
	"fmt"
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

// DRAW-7: sales paid while an admin closes that till for the cashier land in
// the close (counted in its expected) or after it (the next till) — the
// auditor recounts every close from its sales.
func TestDRAW7_SalesDuringACloseAreNeverMissed(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	for round := range 6 {
		s.open(t, s.till1)
		var sess int64
		s.must(t, s.db.Get(&sess, `SELECT id FROM cash_register WHERE closed_at IS NULL`))
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", sess), map[string]any{"breakdown": map[string]int{}}, nil); err != nil {
				t.Errorf("round %d close: %v", round, err)
			}
		}()
		wg.Wait()
	}
	s.audit(t)
}

// DRAW-8: the cashier and an admin closing the same till at the same moment
// close it once and bank the cash once.
func TestDRAW8_TwoClosesAtOnceCloseOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "0")
	for range 5 {
		s.open(t, s.till1)
		var sess int64
		s.must(t, s.db.Get(&sess, `SELECT id FROM cash_register WHERE closed_at IS NULL`))
		body := map[string]any{"breakdown": map[string]int{"1000": 5}, "destLockerId": safe}
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok := 0
		for _, call := range []func() error{
			func() error { return s.till1.Do("POST", "/api/v1/till/drawer/close", body, nil) },
			func() error { return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", sess), body, nil) },
		} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if call() == nil {
					mu.Lock()
					ok++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if ok != 1 {
			t.Fatalf("%d closes went through", ok)
		}
	}
	var bal string
	s.must(t, s.db.Get(&bal, `SELECT SUM(balance_delta)::text FROM locker_ledger WHERE locker_id = $1`, safe))
	if bal != "25000.00" {
		t.Errorf("safe holds %s after five closes of 5,000, want 25000.00", bal)
	}
	s.audit(t)
}

// LOCK-1: transfers out of one no-overdraft locker at the same moment can't
// together take it below zero.
func TestLOCK1_TransfersAtOnceNeverOverdraw(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe, bank := s.locker(t, "Safe", "safe", "3000"), s.locker(t, "Bank", "bank", "0")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": bank, "amount": "1000"}, nil)
		}()
	}
	wg.Wait()
	var bal string
	s.must(t, s.db.Get(&bal, `SELECT SUM(balance_delta)::text FROM locker_ledger WHERE locker_id = $1`, safe))
	if bal != "0.00" {
		t.Errorf("safe holds %s, want 0.00 (three transfers of 1,000 out of 3,000)", bal)
	}
	s.audit(t)
}
