package battle

import (
	"fmt"
	"testing"
	"time"
)

// More of the failure sweep (faults_sweep_test.go): the rest of core's money and
// stock actions. A setup that makes what the action needs runs before every try,
// so an action that can only happen once (void, clear, receive) gets a fresh one.

// last is the newest id in a table, read on the clean connection.
func (s *sweepShop) last(t *testing.T, table string) int64 {
	t.Helper()
	var id int64
	s.must(t, s.look.Get(&id, fmt.Sprintf(`SELECT COALESCE(max(id), 0) FROM %q`, table)))
	return id
}

func TestSweepLockersAndExpenses(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	safe := s.locker(t, "Safe", "safe", "100000")
	bank := s.locker(t, "Bank", "bank", "100000")
	src := fmt.Sprintf("locker:%d", safe)
	s.sweep(t, "expense from the safe", nil, func() error {
		return s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "40", "source": src}, nil)
	})
	var exp int64
	s.sweep(t, "void an expense", func() {
		s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Wrong", "amount": "70", "source": src}, nil))
		exp = s.last(t, "expenses")
	}, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/expenses/%d/void", exp), map[string]any{"dest": src}, nil)
	})
	s.sweep(t, "move between lockers", nil, func() error {
		return s.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": bank, "amount": "200"}, nil)
	})
	for _, typ := range []string{"interest", "bank_charge", "adjust_up"} {
		s.sweep(t, "locker "+typ, nil, func() error {
			return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", bank), map[string]any{"type": typ, "amount": "25", "note": "x"}, nil)
		})
	}
	cust := s.newCustomer(t, "0712000903", "100000", "5000")
	s.sweep(t, "settle the card account", func() {
		s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", cust), map[string]any{"amount": "300", "method": "card",
			"source": fmt.Sprintf("locker:%d", s.cardAcct)}, nil))
	}, func() error {
		return s.admin.Do("POST", "/api/v1/admin/lockers/settle-card", map[string]any{"cardId": s.cardAcct, "bankId": bank, "gross": "300", "fee": "6"}, nil)
	})
}

func TestSweepTillMoney(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	safe := s.locker(t, "Safe", "safe", "100000")
	s.open(t, s.till1)
	s.sweep(t, "pay-in to the drawer", nil, func() error {
		return s.till1.Do("POST", "/api/v1/till/drawer/deposit", map[string]any{"amount": "7.50", "reason": "float", "lockerId": safe}, nil)
	})
	cust := s.newCustomer(t, "0712000904", "100000", "")
	s.sweep(t, "customer pays more than owed", nil, func() error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cust), map[string]any{"amount": "500", "method": "cash"}, nil)
	})
	s.sweep(t, "refund a customer's advance", nil, func() error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", cust), map[string]any{"amount": "10"}, nil)
	})
	s.sweep(t, "change kept on the account", nil, func() error {
		return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"customerId": cust, "items": []line{{"productId": s.prod["plain"], "quantity": "2"}}, "payments": []pay{cash("250")},
			"changeGiven": "30", "onAccountRounding": true}, nil)
	})
	var sess int64
	s.sweep(t, "admin closes a cashier's till", func() {
		_ = s.till1.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 1}}, nil)
		s.must(t, s.look.Get(&sess, `SELECT max(id) FROM cash_register WHERE closed_at IS NULL`))
	}, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", sess), map[string]any{"breakdown": map[string]int{"1000": 1}, "destLockerId": safe}, nil)
	})
}

func TestSweepCheques(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	bank := s.locker(t, "Bank", "bank", "100000")
	sup := s.newSup(t, "Cheque Ltd", "100000")
	n := 0
	var cheque int64
	given := func() {
		n++
		cheque = s.payByCheque(t, sup, bank, "100", fmt.Sprint(500000+n), time.Now().Format(time.DateOnly)) // due today: it may clear
	}
	s.sweep(t, "pay a supplier by cheque", nil, func() error {
		n++
		return s.admin.Do("POST", supPath(sup, "payment"), map[string]any{"amount": "100", "method": "cheque",
			"cheques": []map[string]any{{"lockerId": bank, "number": fmt.Sprint(500000 + n), "amount": "100", "date": "2026-12-01"}}}, nil)
	})
	s.sweep(t, "a given cheque clears", given, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/clear", cheque), map[string]string{}, nil)
	})
	s.sweep(t, "a given cheque bounces", given, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/cheques/%d/return", cheque), map[string]string{"status": "bounced"}, nil)
	})
}

func TestSweepBuyingMore(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	sup := s.newSup(t, "More Ltd", "0")
	safe := s.locker(t, "Safe", "safe", "100000")
	s.sweep(t, "order", nil, func() error {
		return s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup,
			"lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}, nil)
	})
	var po int64
	s.sweep(t, "receive an order", func() {
		var o struct{ ID int64 }
		s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup,
			"lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}, &o))
		po = o.ID
	}, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/purchases/%d/receive", po), map[string]any{"supplierId": sup,
			"lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}, nil)
	})
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "50", "costPrice": "40"}}, nil)
	lot, _, _ := s.lotOf(t, got, s.prod["plain"])
	s.sweep(t, "send back to the supplier", nil, func() error {
		return s.sendBack(t, sup, got, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": lot})
	})
	adv := s.newSup(t, "Advance Ltd", "") // paid before any goods: an advance to refund from
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", adv), map[string]string{"amount": "1000", "method": "cash",
		"source": fmt.Sprintf("locker:%d", safe)}, nil))
	s.sweep(t, "supplier refunds their credit", nil, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/refund", adv), map[string]string{"amount": "10", "method": "cash",
			"source": fmt.Sprintf("locker:%d", safe)}, nil)
	})
}

func TestSweepLossesAndWarranty(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	sup := s.newSup(t, "Loss Ltd", "0")
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "60", "costPrice": "40"}}, nil)
	lot, _, _ := s.lotOf(t, got, s.prod["plain"])
	var mv int64
	s.sweep(t, "a supplier pays for a loss", func() { mv = s.damage(t, s.prod["plain"], lot, "1") }, func() error {
		return s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "10"})
	})
	var rec int64
	s.sweep(t, "undo a recovery", func() {
		mv = s.damage(t, s.prod["plain"], lot, "1")
		s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "10"}))
		rec = s.last(t, "loss_recoveries")
	}, func() error {
		return s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/recovery/%d", rec), nil, nil)
	})
	s.sweep(t, "recall a lot", nil, func() error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", sup, lot), map[string]bool{"stopSelling": true}, nil)
	})
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["serial"], "newQuantity": "200"}, nil))
	s.open(t, s.till1)
	n := 0
	var unit int64
	s.sweep(t, "a warranty swap", func() {
		n++
		serial := fmt.Sprintf("IMEI-SW%d", n)
		s.sell(t, s.till1, "0", []line{{"productId": s.prod["serial"], "quantity": "1", "serials": []string{serial}}}, cash("15000"))
		s.must(t, s.look.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = $1`, serial))
	}, func() error {
		return s.swap(unit, fmt.Sprintf("IMEI-SW%d-B", n))
	})
}
