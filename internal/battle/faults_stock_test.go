package battle

import (
	"fmt"
	"sync"
	"testing"
)

// STK-4 / STK-1: a count saved while sales land never loses one — on hand stays the
// sum of its movements. Rounds, since the window is small.
func TestSTK4_CountWhileSelling(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Counted soap", s.unitPcs, 0, nil)
	s.addLot(t, p, "1000", "10", "")
	s.open(t, s.till1)
	s.open(t, s.till2)
	for round := range 15 {
		atOnce(7, func(i int) error {
			if i == 0 {
				return s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p, "newQuantity": fmt.Sprint(900 - round*10), "note": "count"}, nil)
			}
			c := s.till1
			if i%2 == 0 {
				c = s.till2
			}
			return c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
				"items": []line{{"productId": p, "quantity": "1"}}, "payments": []pay{cash("20")}}, nil)
		})
		s.audit(t)
	}
}

// STK-5: a lot counted while sales take from another lot ends holding its count.
func TestSTK5_LotCountWhileSellingAnotherLot(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Lot soap", s.unitPcs, 0, nil)
	s.addLot(t, p, "500", "10", "2027-01-31")
	s.addLot(t, p, "500", "12", "2027-06-30")
	ids := s.lotIDs(t, p)
	s.open(t, s.till1)
	s.open(t, s.till2)
	for round := range 15 {
		want := fmt.Sprint(480 - round*10)
		atOnce(7, func(i int) error {
			if i == 0 {
				return s.admin.Do("POST", "/api/v1/admin/stock/take", map[string]any{"lines": []map[string]any{{"productId": p,
					"batches": []map[string]any{{"batchId": ids[0], "counted": want}}}}}, nil)
			}
			c := s.till1
			if i%2 == 0 {
				c = s.till2
			}
			return c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
				"items": []line{{"productId": p, "quantity": "1", "batchId": ids[1]}}, "payments": []pay{cash("20")}}, nil)
		})
		if got := s.lotLeft(t, ids[0]); got != want+".000" {
			t.Fatalf("round %d: the counted lot holds %s, counted %s", round, got, want)
		}
		s.audit(t)
	}
}

// STK-23: removals racing for the last units never pass them — write-offs, a box
// opened, units put out for sale; on hand never below zero.
func TestSTK23_RemovalsAtOnceForTheLastUnits(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Last box", s.unitPcs, 0, nil)
	piece := s.stocked(t, "Last piece", s.unitPcs, 0, nil)
	s.addLot(t, p, "5", "100", "")
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": p, "toProductId": piece, "ratio": "10"}, &cv))
	ok := atOnce(12, func(i int) error {
		if i%3 == 0 {
			return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "1"}, nil)
		}
		return s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": "damage"}, nil)
	})
	if ok != 5 || s.onHand(t, p) != "0.000" {
		t.Errorf("%d removals of 5 units went through, on hand %s (want 5, 0)", ok, s.onHand(t, p))
	}
	s.audit(t)
}

// STK-9 / STK-23: a lot given back while sales take from it — never below zero,
// the books still add up.
func TestSTK9_GiveBackWhileSelling(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.supplier(t, "Race Supplier", "0112000006")
	p := s.stocked(t, "Race syrup", s.unitPcs, sup, nil)
	s.addLot(t, p, "10", "50", "2027-01-31")
	s.addLot(t, p, "10", "60", "2027-06-30")
	ids := s.lotIDs(t, p)
	s.open(t, s.till1)
	atOnce(6, func(i int) error {
		if i == 0 {
			return s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/give-back", sup, ids[0]), map[string]string{"counted": "10"}, nil)
		}
		return s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": []line{{"productId": p, "quantity": "1", "batchId": ids[0]}}, "payments": []pay{cash("20")}}, nil)
	})
	s.audit(t)
}

// STK-23 / never stop a sale: picked-lot sales and counts / write-offs naming the
// same lot at the same moment — every sale goes through (no deadlock cancels one),
// the books still add up.
func TestSTK23_PickedLotSalesBesideLotWork(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.stocked(t, "Picked soap", s.unitPcs, 0, nil)
	s.addLot(t, p, "1000", "10", "2027-01-31")
	lot := s.lotIDs(t, p)[0]
	s.open(t, s.till1)
	s.open(t, s.till2)
	for round := range 10 {
		var failed []error
		var mu sync.Mutex
		atOnce(8, func(i int) error {
			var err error
			switch i {
			case 0:
				err = s.admin.Do("POST", "/api/v1/admin/stock/take", map[string]any{"lines": []map[string]any{{"productId": p,
					"batches": []map[string]any{{"batchId": lot, "counted": fmt.Sprint(900 - round*20)}}}}}, nil)
			case 1:
				err = s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": "damage", "batchId": lot}, nil)
			default:
				c := s.till1
				if i%2 == 0 {
					c = s.till2
				}
				if err = c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
					"items": []line{{"productId": p, "quantity": "1", "batchId": lot}}, "payments": []pay{cash("20")}}, nil); err != nil {
					mu.Lock()
					failed = append(failed, err)
					mu.Unlock()
				}
			}
			return err
		})
		if len(failed) > 0 {
			t.Fatalf("round %d: %d sales refused beside a count / write-off of their lot: %v", round, len(failed), failed[0])
		}
		s.audit(t)
	}
}
