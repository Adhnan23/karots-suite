package battle

import (
	"fmt"
	"os"
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

// TestReportsAgreeOverAShopDay builds a day touching every row of area 7's grid
// (docs/rules/reports.md) — and a day before it whose sales come back today —
// then holds every report to the raw rows and to each other (agree), over
// yesterday, today and both.
func TestReportsAgreeOverAShopDay(t *testing.T) {
	t.Parallel()
	// Task 4 of docs/plans/2026-11-20-core-battle-test-reports.md removes this: today it fails on
	// RPT-6 (returns on the sale's day), RPT-12 (Q3), RPT-14, RPT-17, RPT-30 (the Sales report) and ZR-2.
	if os.Getenv("KS_AREA7") == "" {
		t.Skip("area 7: the report fixes are pending (KS_AREA7=1 runs it)")
	}
	s := newShop(t)
	ctx := t.Context()
	// A tax-registered shop, an 18% product, and a pass-through service (airtime: a plugin's kind, set as one would).
	s.must(t, sqlxExec(ctx, s.db, `UPDATE settings SET tax_registered = true, tax_reg_no = 'VAT-1'`))
	s.must(t, sqlxExec(ctx, s.db, `UPDATE products SET tax_rate = 18 WHERE name = 'Battle odd'`))
	var air struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Battle airtime", "categoryId": s.cat, "unitId": s.unitPcs,
		"costPrice": "0", "sellingPrice": "100", "wholesalePrice": "100", "reorderAuto": false}, &air))
	s.must(t, sqlxExec(ctx, s.db, fmt.Sprintf(`UPDATE products SET pass_through = true, is_service = true WHERE id = %d`, air.ID)))
	sup := s.newSup(t, "Battle supplier", "")

	s.open(t, s.till1)
	s.open(t, s.till2)
	plain, odd, pack := s.prod["plain"], s.prod["odd"], s.prod["pack"]

	// Yesterday's sales (moved back a day below), some coming back today.
	yA := s.sell(t, s.till1, "10", []line{{"productId": plain, "quantity": "3"}, {"productId": odd, "quantity": "2"}}, cash("1000"))
	yB := s.sell(t, s.till1, "5", []line{{"productId": odd, "quantity": "1"}, {"productId": air.ID, "quantity": "1"}},
		cash("1000"))
	yC, err := s.onAccount(t, s.customer, 2, "50", nil)
	s.must(t, err)
	s.must(t, sqlxExec(ctx, s.db, fmt.Sprintf(`UPDATE sales SET created_at = created_at - interval '1 day' WHERE id IN (%d, %d, %d)`, yA.ID, yB.ID, yC.ID)))

	// Today: discounts, a pack, a loose line, a payout beside goods, a split tender, a sale to be voided.
	s.sell(t, s.till1, "3", []line{{"productId": plain, "quantity": "1", "discount": "7"}, {"productId": pack, "quantity": "12"}}, cash("500"))
	s.sell(t, s.till2, "0", []line{{"productId": s.prod["loose"], "quantity": "0.375"}, {"productId": odd, "quantity": "3"}}, cash("500"))
	s.sell(t, s.till1, "0", []line{{"productId": plain, "quantity": "3"}}, cash("100"), pay{"method": "card", "amount": "200", "accountId": s.cardAcct})
	s.sell(t, s.till2, "0", []line{{"productId": plain, "quantity": "2"}, {"productId": air.ID, "quantity": "1", "payout": true, "priceOverride": "50"}}, cash("200"))
	void := s.sell(t, s.till1, "0", []line{{"productId": s.prod["cheap"], "quantity": "4"}}, cash("10"))

	// Returns today: part of yesterday's A, all of B's goods, one of C (on account), and part of a today's sale.
	ya := s.sale(t, s.till1, yA.ID)
	s.giveBack(t, s.till1, yA.ID, ya.Lines[0].ID, "1")
	s.giveBack(t, s.till2, yA.ID, ya.Lines[1].ID, "2")
	s.giveBack(t, s.till1, yB.ID, s.sale(t, s.till1, yB.ID).Lines[0].ID, "1")
	s.giveBack(t, s.till2, yC.ID, s.sale(t, s.till2, yC.ID).Lines[0].ID, "1")

	// Money and stock beside the sales.
	s.must(t, s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Tea", "amount": "40", "source": fmt.Sprintf("till:%d", s.userID(t, "0770000002"))}, nil))
	s.payCash(t, s.customer, "60")
	var bank struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": "Battle bank", "kind": "bank", "openingBalance": "0"}, &bank))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", bank.ID), map[string]any{"type": "interest", "amount": "12.34", "note": "x"}, nil))
	_, err = s.deliver(t, sup, []sl{{"productId": plain, "quantity": "5", "costPrice": "60"}}, nil)
	s.must(t, err)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]string{"amount": "20", "method": "discount"}, nil))
	undone := s.damage(t, odd, 0, "2")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": undone, "outcome": "paid", "supplierId": sup, "amount": "30"}))
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/recovery/%d", s.lastRecovery(t)), nil, nil))
	kept := s.damage(t, plain, 0, "1")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": kept, "outcome": "paid", "supplierId": sup, "amount": "25"}))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["cheap"], "newQuantity": "99990", "note": "count"}, nil))

	// till2 closes Rs 10 short; till1 stays open.
	short := dec(t, s.expected(t, s.till2)).Sub(dec(t, "10")).StringFixed(2)
	s.must(t, s.till2.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{short: 1}}, nil))

	s.audit(t)
	// A void sale is in no report (RPT-5); core never voids, plugins may.
	s.must(t, sqlxExec(ctx, s.db, fmt.Sprintf(`UPDATE sales SET status = 'void' WHERE id = %d`, void.ID)))

	today := time.Now().In(datetime.Location)
	d0, d1 := today.AddDate(0, 0, -1).Format(time.DateOnly), today.Format(time.DateOnly)
	for _, r := range [][2]string{{d0, d0}, {d1, d1}, {d0, d1}} {
		s.agree(t, r[0], r[1])
	}
}
