package battle

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Area 6 faults (docs/rules/recovery.md): two swaps of one fault, two
// settlements or undos of one loss, and recoveries beside sales, deliveries,
// payments and returns.

// WAR-6: eight swaps of one serial unit at once hand out one unit; eight
// by-receipt swaps of a line's last unit, one.
func TestWAR6_SwapsAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph, p := s.prod["serial"], s.prod["plain"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "20"}, nil))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE products SET warranty_months = 6 WHERE id = %d`, p)))
	s.open(t, s.till1)
	unit := s.phoneSold(t, "W6-A", 0)
	if ok := atOnce(8, func(i int) error { return s.swap(unit, fmt.Sprintf("W6-A-%d", i)) }); ok != 1 {
		t.Errorf("WAR-6 %d serial swaps of one unit went through, want 1", ok)
	}
	if n := s.count(t, `SELECT count(*) FROM stock_movements WHERE type = 'warranty_replacement' AND product_id = $1`, ph); n != 1 {
		t.Errorf("WAR-6 %d units handed out for one fault", n)
	}
	r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "1"}}, cash("100"))
	if ok := atOnce(8, func(int) error { return s.swapByReceipt(r.ID, p) }); ok != 1 {
		t.Errorf("WAR-6 %d by-receipt swaps of one unit went through, want 1", ok)
	}
	s.audit(t)
}

// WAR-6 / lock order: by-receipt swaps beside returns of the same sale never
// deadlock; together they never pass what was sold.
func TestWAR6_SwapsBesideReturns(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.prod["plain"]
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE products SET warranty_months = 6 WHERE id = %d`, p)))
	s.open(t, s.till1)
	var mu sync.Mutex
	var errs []string
	for range 6 {
		r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "4"}}, cash("400"))
		item := s.sale(t, s.till1, r.ID).Lines[0].ID
		atOnce(8, func(i int) error {
			var err error
			if i%2 == 0 {
				err = s.swapByReceipt(r.ID, p)
			} else {
				err = s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{
					"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}, nil)
			}
			if err != nil {
				mu.Lock()
				errs = append(errs, err.Error())
				mu.Unlock()
			}
			return err
		})
	}
	for _, e := range errs {
		if strings.Contains(e, "deadlock") || strings.Contains(e, "500") {
			t.Errorf("refused by the database: %s", e)
		}
	}
	s.audit(t) // WAR-4: swaps ≤ sold − returned
}

// REC-1: eight settlements of one loss at once — one.
func TestREC1_SettlementsAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R1 Race", "")
	mv := s.damage(t, s.prod["odd"], 0, "1")
	ok := atOnce(8, func(i int) error {
		return s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "10"})
	})
	if got := s.supBal(t, sup); ok != 1 || got != "-10.00" {
		t.Errorf("REC-1 %d settlements went through, owed %s (want 1, -10.00)", ok, got)
	}
	s.audit(t)
}

// REC-8: eight undos of one recovery at once — one.
func TestREC8_UndosAtOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R8 Race", "")
	p := s.stocked(t, "R8 Race lamp", s.unitPcs, sup, nil)
	lot, _, _ := s.lotOf(t, s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "10"}}, nil), p)
	mv := s.damage(t, p, lot, "2")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "supplierId": sup, "batchId": lot, "extraPerUnit": "1"}))
	rec := s.lastRecovery(t)
	if ok := atOnce(8, func(int) error { return s.undo(rec) }); ok != 1 {
		t.Errorf("REC-8 %d undos went through, want 1", ok)
	}
	if got, left := s.supBal(t, sup), s.lotLeft(t, lot); got != "100.00" || left != "8.000" {
		t.Errorf("after undo: owed %s, lot holds %s (want 100.00, 8.000)", got, left)
	}
	s.audit(t)
}

// REC-7: recoveries (replacements with an extra, credits, undos) beside sales of
// the same product from two tills, deliveries and payments to the same
// supplier: a sale, a delivery or a payment is never refused, and the books hold.
func TestREC7_RecoveriesBesideEverything(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "1000000"))
	sup := s.newSup(t, "R7 Busy", "500")
	p := s.stocked(t, "R7 Busy soap", s.unitPcs, sup, nil)
	lot, _, _ := s.lotOf(t, s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "2000", "costPrice": "10"}}, nil), p)
	s.open(t, s.till1)
	s.open(t, s.till2)
	var mu sync.Mutex
	var errs []string
	for range 8 {
		losses := []int64{s.damage(t, p, lot, "1"), s.damage(t, p, lot, "1"), s.damage(t, p, lot, "1")}
		atOnce(9, func(i int) (err error) {
			defer func() {
				if err != nil && i < 6 {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%d: %v", i, err))
					mu.Unlock()
				}
			}()
			sale := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
				"items": []line{{"productId": p, "quantity": "1"}, {"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("200")}}
			switch i {
			case 0:
				return s.till1.Do("POST", "/api/v1/till/sales", sale, nil)
			case 1:
				return s.till2.Do("POST", "/api/v1/till/sales", sale, nil)
			case 2:
				_, err = s.deliver(t, sup, []sl{{"productId": p, "quantity": "3", "costPrice": "10"}}, nil)
				return err
			case 3:
				return s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "25", "method": "cash", "source": safe}, nil)
			case 4:
				return s.recover(map[string]any{"sourceType": "damage", "sourceId": losses[0], "outcome": "replacement", "supplierId": sup, "batchId": lot, "extraPerUnit": "2"})
			case 5:
				return s.recover(map[string]any{"sourceType": "damage", "sourceId": losses[1], "outcome": "paid", "supplierId": sup, "amount": "40"})
			case 6:
				return s.recover(map[string]any{"sourceType": "damage", "sourceId": losses[2], "outcome": "replacement", "expiry": "2031-01-01"})
			default:
				// An undo of the newest recovery: refused once anything came after.
				var id int64
				if s.db.Get(&id, `SELECT COALESCE(MAX(id), 0) FROM loss_recoveries`) == nil && id > 0 {
					return s.undo(id)
				}
				return nil
			}
		})
	}
	for _, e := range errs {
		t.Errorf("refused or failed: %s", e)
	}
	s.audit(t)
}
