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
	var other struct{ ID int64 } // one cashier's sales, and sales of one product, already wait for each other
	s.must(t, s.admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Battle other", "categoryId": s.cat, "unitId": s.unitPcs,
		"costPrice": "1", "sellingPrice": "100", "wholesalePrice": "100", "reorderAuto": false}, &other))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": other.ID, "newQuantity": "1000", "note": "battle"}, nil))
	ok := atOnce(8, func(i int) error {
		return []*client{s.till1, s.till2}[i%2].Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0",
			"discountType": "fixed", "customerId": id, "items": []line{{"productId": []int64{s.prod["plain"], other.ID}[i%2], "quantity": "2"}},
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
