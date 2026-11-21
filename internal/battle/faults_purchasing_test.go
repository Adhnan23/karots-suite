package battle

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Area 5 faults (docs/rules/purchasing.md): two actions on one order, one
// supplier or one cheque at the same moment, and deliveries beside sales.

// PUR-6: eight receives of one order at once book it once.
func TestPUR6_ReceivesAtOnceBookOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Race Ltd", "")
	var o struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup, "lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}, &o))
	before := s.onHand(t, s.prod["plain"])
	ok := atOnce(8, func(int) error {
		return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/purchases/%d/receive", o.ID), map[string]any{"supplierId": sup,
			"lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}, nil)
	})
	if got := s.supBal(t, sup); ok != 1 || got != "500.00" {
		t.Errorf("PUR-6 %d receives went through, owed %s (want 1, 500.00)", ok, got)
	}
	if d := decimal.RequireFromString(s.onHand(t, s.prod["plain"])).Sub(decimal.RequireFromString(before)); !d.Equal(decimal.NewFromInt(10)) {
		t.Errorf("PUR-6 stock up by %s, want 10", d)
	}
	s.audit(t)
}

// PUR-5 / PUR-6: edits of an order beside its receive — the bill, once received,
// is never turned back into an order.
func TestPUR6_EditBesideReceive(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Edit Race Ltd", "")
	for range 6 {
		var o struct{ ID int64 }
		body := map[string]any{"supplierId": sup, "lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}
		s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", body, &o))
		atOnce(6, func(i int) error {
			if i%2 == 0 {
				return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/purchases/%d/receive", o.ID), body, nil)
			}
			return s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/purchases/%d", o.ID), body, nil)
		})
		if b := s.bill(t, o.ID); b.Status == "draft" {
			t.Errorf("PUR-5 order %d received, then edited back into an order", o.ID)
		}
	}
	if got := s.supBal(t, sup); got != "3000.00" {
		t.Errorf("owed %s, want 3000.00 (six orders received once each)", got)
	}
	s.audit(t)
}

// SUP-7: of eight Rs 100 refunds at once against a Rs 300 credit, three go through.
func TestSUP7_RefundsAtOnceNeverPassTheCredit(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "10000"))
	sup := s.newSup(t, "Credit Race Ltd", "")
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "300", "method": "cash", "source": safe}, nil))
	ok := atOnce(8, func(int) error {
		return s.admin.Do("POST", supPath(sup, "refund"), map[string]string{"amount": "100", "method": "cash", "source": safe}, nil)
	})
	if got := s.supBal(t, sup); ok != 3 || got != "0.00" {
		t.Errorf("SUP-7 %d refunds went through, balance %s (want 3, 0.00)", ok, got)
	}
	s.audit(t)
}

// SUP-8: of eight Rs 100 old-debt payments at once against Rs 300 of old debt,
// three go through, and only their money leaves.
func TestSUP8_OldDebtPaymentsAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "10000")
	sup := s.newSup(t, "Old Race Ltd", "300")
	ok := atOnce(8, func(int) error {
		return s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"oldDebt": "100", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}, nil)
	})
	bal, _, old := s.supState(t, sup)
	if ok != 3 || !bal.IsZero() || !old.IsZero() {
		t.Errorf("SUP-8 %d old-debt payments, owed %s, old debt %s (want 3, 0, 0)", ok, bal, old)
	}
	eq(t, "safe", s.lockerBal(t, safe), "9700")
	s.audit(t)
}

// SUP-8: payments, refunds, an opening adjusted and deliveries settled from the
// supplier's credit, all at once — each lands whole; nothing is spent twice.
func TestSUP8_MoneyActionsAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "1000000"))
	sup := s.newSup(t, "Busy Ltd", "200")
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "2000", "method": "cash", "source": safe}, nil))
	var mu sync.Mutex
	var errs []string
	for round := range 8 {
		atOnce(6, func(i int) (err error) {
			defer func() {
				if err != nil {
					mu.Lock()
					errs = append(errs, err.Error())
					mu.Unlock()
				}
			}()
			switch i {
			case 0, 1:
				_, err = s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "3", "costPrice": "100"}}, nil)
			case 2:
				err = s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "150", "method": "cash", "source": safe}, nil)
			case 3:
				err = s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "50", "method": "cash", "mode": "old", "source": safe}, nil)
			case 4:
				err = s.admin.Do("POST", supPath(sup, "opening"), map[string]string{"opening": fmt.Sprint(100 + 25*round)}, nil)
			default:
				// Refused once their credit is spent, as it should be; the auditor
				// holds what did go through.
				s.admin.Do("POST", supPath(sup, "refund"), map[string]string{"amount": "10", "method": "cash", "source": safe}, nil) //nolint:errcheck
			}
			return err
		})
	}
	for _, e := range errs {
		t.Errorf("refused or failed: %s", e)
	}
	s.audit(t)
}

// CHQ-6: a clear and a bounce of one cheque at once — one wins.
func TestCHQ6_ClearAndBounceAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	bank := s.locker(t, "Bank", "bank", "10000")
	sup := s.newSup(t, "Cheque Race Ltd", "1000")
	today := time.Now().Format("2006-01-02")
	for n := range 5 {
		c := s.payByCheque(t, sup, bank, "100", fmt.Sprintf("74000%d", n), today)
		ok := atOnce(4, func(i int) error {
			if i%2 == 0 {
				return s.admin.Do("POST", s.chequeTo(c, "clear"), map[string]string{}, nil)
			}
			return s.admin.Do("POST", s.chequeTo(c, "return"), map[string]string{"status": "bounced"}, nil)
		})
		if ok != 1 {
			t.Errorf("CHQ-6 %d of a clear and a bounce went through, want 1", ok)
		}
	}
	s.audit(t)
}

// PUR-2: deliveries of a product beside sales of it, a supplier paid from a till
// and the counter's walk-in delivery paid from the drawer, beside sales on that till — every one goes through (lock order: the
// till, stock, lots, then the supplier).
func TestPUR2_DeliveriesBesideSales(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.must(t, sqlxExec(context.Background(), s.db, `UPDATE users SET can_handle_suppliers = true`))
	till := fmt.Sprintf("till:%d", s.userID(t, "0770000002"))
	sup := s.newSup(t, "Beside Ltd", "100000")
	failed := 0
	var errs sync.Map
	for range 8 {
		n := atOnce(8, func(i int) (err error) {
			defer func() {
				if err != nil {
					errs.Store(err.Error(), true)
				}
			}()
			switch i % 4 {
			case 0:
				_, err = s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "2", "costPrice": "40"}, {"productId": s.prod["odd"], "quantity": "2", "costPrice": "10"}}, nil)
				return err
			case 1:
				return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
					"items": []line{{"productId": s.prod["odd"], "quantity": "1"}, {"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("200")}}, nil)
			case 2:
				return s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "20", "method": "cash", "source": till}, nil)
			default:
				return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/suppliers/%d/receive", sup), map[string]any{"discountType": "fixed",
					"lines": []sl{{"productId": s.prod["odd"], "quantity": "1", "costPrice": "10", "discountType": "fixed"}, {"productId": s.prod["plain"], "quantity": "1", "costPrice": "40", "discountType": "fixed"}},
					"pay":   map[string]string{"amount": "50", "method": "cash"}}, nil)
			}
		})
		failed += 8 - n
	}
	if failed > 0 {
		errs.Range(func(k, _ any) bool { t.Log(k); return true })
		t.Errorf("%d of 64 refused or failed", failed)
	}
	s.audit(t)
}
