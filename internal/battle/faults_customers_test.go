package battle

import (
	"fmt"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

// atOnce runs do n times at the same moment; how many went through.
func atOnce(n int, do func(i int) error) int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if do(i) == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return ok
}

// CUST-11: of eight Rs 100 advance refunds at once against a Rs 300 advance, three go through.
func TestCUST11_RefundsAtOnceNeverPassTheAdvance(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "10000")
	id := s.newCustomer(t, "0712000201", "0", "0")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", id), map[string]any{"amount": "300", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	ok := atOnce(8, func(int) error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/refund", id), map[string]any{"amount": "100", "source": fmt.Sprintf("locker:%d", safe)}, nil)
	})
	if bal := s.acct(t, id).Balance; ok != 3 || !bal.IsZero() {
		t.Errorf("CUST-11 %d refunds went through, balance %s (want 3, 0)", ok, bal)
	}
	s.audit(t)
}

// CUST-13 / CUST-6 / CUST-9: eight old-debt-first payments at once all count, and
// every slip's before is the balance the one before it left.
func TestCUST13_PaymentsAtOnceAllCount(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	safe := s.locker(t, "Safe", "safe", "0")
	id := s.newCustomer(t, "0712000211", "100000", "300")
	_, err := s.onAccount(t, id, 1, "0", nil)
	s.must(t, err)
	ok := atOnce(8, func(int) error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", id), map[string]any{"amount": "50", "method": "cash",
			"source": fmt.Sprintf("locker:%d", safe), "oldDebtFirst": true}, nil)
	})
	a := s.acct(t, id)
	if ok != 8 || !a.Balance.IsZero() || !a.OldDebt.IsZero() {
		t.Errorf("CUST-13 %d payments, balance %s, old debt %s (want 8, 0, 0)", ok, a.Balance, a.OldDebt)
	}
	var rows []struct {
		Before decimal.Decimal `db:"balance_before"`
		After  decimal.Decimal `db:"balance_after"`
	}
	s.must(t, s.db.Select(&rows, `SELECT balance_before, balance_after FROM customer_payments WHERE customer_id = $1 ORDER BY id`, id))
	for i := 1; i < len(rows); i++ {
		if !rows[i].Before.Equal(rows[i-1].After) {
			t.Errorf("CUST-6 slip %d says before %s, the slip before left %s", i, rows[i].Before, rows[i-1].After)
		}
	}
	s.audit(t)
}

// CUST-8 / TEND-12: of eight Rs 200 sales on account at once against a Rs 1,000
// limit, five go through, and each receipt's balance after is its own.
func TestCUST8_CreditSalesAtOnceKeepTheLimit(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	id := s.newCustomer(t, "0712000221", "1000", "0")
	other := s.otherProduct(t) // one cashier's sales, and sales of one product, already wait for each other
	ok := atOnce(8, func(i int) error {
		return []*client{s.till1, s.till2}[i%2].Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0",
			"discountType": "fixed", "customerId": id, "items": []line{{"productId": []int64{s.prod["plain"], other}[i%2], "quantity": "2"}},
			"payments": []pay{{"method": "credit", "amount": "200"}}}, nil)
	})
	var after []decimal.Decimal
	s.must(t, s.db.Select(&after, `SELECT customer_balance_after FROM sales WHERE customer_id = $1 ORDER BY id`, id))
	if bal := s.acct(t, id).Balance; ok != 5 || !bal.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("CUST-8 %d sales on account, balance %s (want 5, 1000)", ok, bal)
	}
	for i, a := range after {
		if !a.Equal(decimal.NewFromInt(int64(200 * (i + 1)))) {
			t.Errorf("TEND-12 receipt %d says balance after %s, want %d", i, a, 200*(i+1))
		}
	}
	s.audit(t)
}

// CUST-16: the same Pay sent four times at the same moment is one payment.
func TestCUST16_SamePayAtOnceIsOne(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000231", "0", "1000")
	ok := atOnce(4, func(int) error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", id), map[string]any{"amount": "100", "method": "cash", "clientRef": "same-pay"}, nil)
	})
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM customer_payments WHERE customer_id = $1`, id))
	if bal := s.acct(t, id).Balance; ok != 4 || n != 1 || !bal.Equal(decimal.NewFromInt(900)) {
		t.Errorf("CUST-16 %d answered, %d payments, balance %s (want 4, 1, 900)", ok, n, bal)
	}
	s.audit(t)
}

// RET-19 / CUST-1: two returns at once, on two credit sales of a customer who now
// owes only one of them, clear that debt once; the other comes back as cash.
func TestRET19_ReturnsAtOnceClearDebtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	id := s.newCustomer(t, "0712000241", "100000", "0")
	var lines [2][2]int64 // sale, line — two products, so stock doesn't make them wait
	for i, p := range []int64{s.prod["plain"], s.otherProduct(t)} {
		var r saleResult
		s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"customerId": id, "items": []line{{"productId": p, "quantity": "1"}}, "payments": []pay{{"method": "credit", "amount": "100"}}}, &r))
		lines[i] = [2]int64{r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID}
	}
	s.payCash(t, id, "100") // owes 100 now: one sale's worth
	var cash [2]decimal.Decimal
	atOnce(2, func(i int) error {
		var r struct{ CashRefund string }
		err := []*client{s.till1, s.till2}[i].Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", lines[i][0]), map[string]any{
			"lines": []map[string]any{{"saleItemId": lines[i][1], "quantity": "1"}}}, &r)
		if err == nil {
			cash[i] = dec(t, r.CashRefund)
		}
		return err
	})
	if bal := s.acct(t, id).Balance; !bal.IsZero() || !cash[0].Add(cash[1]).Equal(decimal.NewFromInt(100)) {
		t.Errorf("RET-19 balance %s, cash back %s + %s (want 0, 100 in all)", bal, cash[0], cash[1])
	}
	s.audit(t)
}

// otherProduct makes a second Rs 100 product with stock; its id.
func (s *shop) otherProduct(t *testing.T) int64 {
	var p struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Battle other", "categoryId": s.cat, "unitId": s.unitPcs,
		"costPrice": "1", "sellingPrice": "100", "wholesalePrice": "100", "reorderAuto": false}, &p))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p.ID, "newQuantity": "1000", "note": "battle"}, nil))
	return p.ID
}
