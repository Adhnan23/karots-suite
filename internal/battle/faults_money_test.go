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

// DRAW-7 / DRAW-6: sales paid while an admin closes that till for the cashier,
// who opens again at once, land in the close (counted in its expected) or in
// the next till — never in none. The auditor recounts every close from its
// sales and finds any sale left with no till.
func TestDRAW7_SalesDuringACloseAreNeverMissed(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	for round := range 8 {
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
			s.open(t, s.till1)
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

// SALE-14 / DRAW-3 (never stop a sale): one cashier selling and giving a cash
// refund at the same moment (two windows) — both always go through.
func TestDRAW3_SaleAndRefundAtOnceBothGoThrough(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	for round := range 30 {
		r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("200"))
		item := s.sale(t, s.till1, r.ID).Lines[0].ID
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
				"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("100")}}, nil); err != nil {
				t.Errorf("round %d sale: %v", round, err)
			}
		}()
		go func() {
			defer wg.Done()
			if err := s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
				"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, nil); err != nil {
				t.Errorf("round %d refund: %v", round, err)
			}
		}()
		wg.Wait()
	}
	s.audit(t)
}

// DRAW-3 / never stop a sale: several sales beside a cash refund on the same till,
// many rounds — a sale takes the till then stock, so the refund must too, or the
// database breaks the deadlock by refusing one.
func TestDRAW3_SalesBesideRefundsNeverDeadlock(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	for round := range 25 {
		r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "3"}}, cash("300"))
		item := s.sale(t, s.till1, r.ID).Lines[0].ID
		var mu sync.Mutex
		var failed []string
		atOnce(6, func(i int) error {
			var err error
			if i < 2 {
				err = s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
					"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, nil)
			} else {
				err = s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
					"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("100")}}, nil)
			}
			if err != nil {
				mu.Lock()
				failed = append(failed, err.Error())
				mu.Unlock()
			}
			return err
		})
		if len(failed) > 0 {
			t.Fatalf("round %d: %v", round, failed)
		}
	}
	s.audit(t)
}
