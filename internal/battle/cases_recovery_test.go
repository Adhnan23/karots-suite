package battle

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// phoneSold sells one serial phone (with the customer when cust > 0) and
// answers its warranty unit.
func (s *shop) phoneSold(t *testing.T, serial string, cust int64) int64 {
	t.Helper()
	body := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": []line{{"productId": s.prod["serial"], "quantity": "1", "serials": []string{serial}}}, "payments": []pay{cash("15000")}}
	if cust > 0 {
		body["customerId"] = cust
	}
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", body, nil))
	var unit int64
	s.must(t, s.db.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = $1 AND status = 'active'`, serial))
	return unit
}

func (s *shop) swap(unit int64, serial string) error {
	return s.till1.Do("POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": unit, "newSerial": serial, "reason": "dead"}, nil)
}

func (s *shop) swapByReceipt(sale, product int64) error {
	return s.till1.Do("POST", "/api/v1/till/warranty/receipt-replace", map[string]any{"saleId": sale, "productId": product, "reason": "cracked"}, nil)
}

func (s *shop) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	s.must(t, s.db.Get(&n, q, args...))
	return n
}

// damage writes qty of product p off (lot given, or FEFO when 0) and answers the movement.
func (s *shop) damage(t *testing.T, p, lot int64, qty string) int64 {
	t.Helper()
	body := map[string]any{"productId": p, "quantity": qty, "reason": "damage"}
	if lot > 0 {
		body["batchId"] = lot
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", body, nil))
	var mv int64
	s.must(t, s.db.Get(&mv, `SELECT id FROM stock_movements WHERE product_id = $1 AND type = 'damage' ORDER BY id DESC LIMIT 1`, p))
	return mv
}

func (s *shop) recover(body map[string]any) error {
	return s.admin.Do("POST", "/api/v1/admin/recovery", body, nil)
}

// lastRecovery is the newest recovery's id.
func (s *shop) lastRecovery(t *testing.T) int64 {
	t.Helper()
	var id int64
	s.must(t, s.db.Get(&id, `SELECT MAX(id) FROM loss_recoveries`))
	return id
}

func (s *shop) undo(id int64) error {
	return s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/recovery/%d", id), nil, nil)
}

// WAR-1: only a unit under warranty is swapped — never a returned one, an
// out-of-date one or one already swapped; the last day still counts.
func TestWAR1OnlyUnderWarranty(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph := s.prod["serial"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "10"}, nil))
	s.open(t, s.till1)

	back := s.phoneSold(t, "W1-RET", 0)
	var sale int64
	s.must(t, s.db.Get(&sale, `SELECT sale_id FROM warranty_units WHERE id = $1`, back))
	s.giveBack(t, s.till1, sale, s.sale(t, s.till1, sale).Lines[0].ID, "1")
	s.mustRefuse(t, "swap a returned unit", s.swap(back, "W1-RET-B"))

	late := s.phoneSold(t, "W1-LATE", 0)
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE warranty_units SET warranty_until = CURRENT_DATE - 1 WHERE id = %d`, late)))
	s.mustRefuse(t, "swap a unit past its last day", s.swap(late, "W1-LATE-B"))

	last := s.phoneSold(t, "W1-LAST", 0)
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE warranty_units SET warranty_until = '%s' WHERE id = %d`, time.Now().Format(time.DateOnly), last)))
	if err := s.swap(last, "W1-LAST-B"); err != nil {
		t.Errorf("swap on the last day: %v", err)
	}
	s.mustRefuse(t, "swap a unit already swapped", s.swap(last, "W1-LAST-C"))
	if n := s.count(t, `SELECT count(*) FROM warranty_claims`); n != 1 {
		t.Errorf("%d claims, want only the last-day one", n)
	}
	s.audit(t)
}

// WAR-2: a new serial nobody has; the same customer; the warranty continues.
func TestWAR2SwapContinuesTheWarranty(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph := s.prod["serial"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "10"}, nil))
	s.open(t, s.till1)
	other := s.phoneSold(t, "W2-OTHER", 0)
	unit := s.phoneSold(t, "W2-A", s.customer)
	s.mustRefuse(t, "a serial another unit has", s.swap(unit, "W2-OTHER"))
	s.must(t, s.swap(unit, "W2-B"))
	var until, newUntil string
	var cust, newCust *int64
	s.must(t, s.db.QueryRow(`SELECT o.warranty_until::text, o.customer_id, n.warranty_until::text, n.customer_id
		FROM warranty_units o JOIN warranty_units n ON n.id = o.replaced_by_unit_id WHERE o.id = $1`, unit).Scan(&until, &cust, &newUntil, &newCust))
	if until != newUntil || cust == nil || newCust == nil || *cust != *newCust {
		t.Errorf("until %s → %s, customer %v → %v", until, newUntil, cust, newCust)
	}
	_ = other
	s.audit(t)
}

// WAR-3: the unit handed out is the next lot's, at its cost; a refused swap saves nothing.
func TestWAR3SwapTakesTheNextLot(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ph := s.stocked(t, "W3 Phone", s.unitPcs, 0, map[string]any{"trackSerial": true, "warrantyMonths": 12})
	s.addLot(t, ph, "1", "900", time.Now().AddDate(1, 0, 0).Format(time.DateOnly))
	s.addLot(t, ph, "1", "700", time.Now().AddDate(0, 6, 0).Format(time.DateOnly))
	s.open(t, s.till1)
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": []line{{"productId": ph, "quantity": "1", "serials": []string{"W3-A"}}}, "payments": []pay{cash("20")}}, nil))
	var unit int64
	s.must(t, s.db.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = 'W3-A'`))
	s.must(t, s.swap(unit, "W3-B"))
	if typ, q, w := s.lastMove(t, ph); typ != "warranty_replacement" || !q.Equal(decimal.NewFromInt(-1)) || !w.Equal(decimal.NewFromInt(900)) {
		t.Errorf("swap moved %s %s worth %s, want the 900 lot (the 700 one sold first)", typ, q, w)
	}
	s.must(t, s.db.Get(&unit, `SELECT id FROM warranty_units WHERE serial_no = 'W3-B'`))
	claims := s.count(t, `SELECT count(*) FROM warranty_claims`)
	s.mustRefuse(t, "swap with none left", s.swap(unit, "W3-C"))
	if n := s.count(t, `SELECT count(*) FROM warranty_claims`); n != claims || s.count(t, `SELECT count(*) FROM warranty_units WHERE serial_no = 'W3-C'`) != 0 {
		t.Errorf("a refused swap left a claim or a unit")
	}
	s.audit(t)
}

// WAR-4: by receipt, swaps never pass the units sold less the units returned; a void sale has none.
func TestWAR4ByReceiptNeverPastWhatsKept(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	p := s.prod["plain"]
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE products SET warranty_months = 6 WHERE id = %d`, p)))
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "2"}, {"productId": p, "quantity": "1"}}, cash("300"))
	s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "1")
	s.must(t, s.swapByReceipt(r.ID, p))
	s.must(t, s.swapByReceipt(r.ID, p))
	s.mustRefuse(t, "a third swap when one of three came back", s.swapByReceipt(r.ID, p))

	// The last day of cover still counts; the day after doesn't.
	l := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "2"}}, cash("200"))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET created_at = CURRENT_DATE + interval '12 hours' - interval '6 months' WHERE id = %d`, l.ID)))
	s.must(t, s.swapByReceipt(l.ID, p))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET created_at = CURRENT_DATE + interval '12 hours' - interval '6 months 1 day' WHERE id = %d`, l.ID)))
	s.mustRefuse(t, "a swap the day after the cover ended", s.swapByReceipt(l.ID, p))

	v := s.sell(t, s.till1, "0", []line{{"productId": p, "quantity": "1"}}, cash("100"))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET status = 'void' WHERE id = %d`, v.ID)))
	s.mustRefuse(t, "a swap on a void sale", s.swapByReceipt(v.ID, p))
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE sales SET status = 'completed' WHERE id = %d`, v.ID)))
	s.audit(t)
}

// WAR-5: a swap is no sale and no money.
func TestWAR5SwapIsNoMoney(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["serial"], "newQuantity": "5"}, nil))
	s.open(t, s.till1)
	unit := s.phoneSold(t, "W5-A", s.customer)
	sales, drawer, owed := s.count(t, `SELECT count(*) FROM sales`), s.expected(t, s.till1), s.acct(t, s.customer).Balance
	s.must(t, s.swap(unit, "W5-B"))
	if s.count(t, `SELECT count(*) FROM sales`) != sales || s.expected(t, s.till1) != drawer || !s.acct(t, s.customer).Balance.Equal(owed) {
		t.Errorf("a swap moved a sale, the drawer or the customer")
	}
	s.audit(t)
}

// REC-1: a loss is settled once; an unswapped unit has nothing to settle.
func TestREC1SettledOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	mv := s.damage(t, s.prod["odd"], 0, "1")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "written_off"}))
	s.mustRefuse(t, "settle a loss twice", s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "written_off"}))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["serial"], "newQuantity": "1"}, nil))
	s.open(t, s.till1)
	unit := s.phoneSold(t, "R1-A", 0)
	s.mustRefuse(t, "settle a unit never swapped", s.recover(map[string]any{"sourceType": "warranty", "sourceId": unit, "outcome": "written_off"}))
	s.audit(t)
}

// REC-2: a replacement comes back worth the loss + the extra; a top-up blends the lot's cost.
func TestREC2ReplacementComesBackAtItsWorth(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R2 Supplies", "")
	p := s.stocked(t, "R2 Bulb", s.unitPcs, sup, nil)
	id := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "10"}}, nil)
	lot, _, _ := s.lotOf(t, id, p)
	bal := s.supBal(t, sup)

	// Into a new lot, Rs 2 a unit extra, at a new price.
	mv := s.damage(t, p, lot, "3")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "supplierId": sup,
		"expiry": "2030-01-01", "extraPerUnit": "2", "sellingPrice": "25"}))
	rec := s.lastRecovery(t)
	var ref *int64
	var q, worth, newCost, price decimal.Decimal
	s.must(t, s.db.QueryRow(`SELECT reference_id, quantity, cost FROM stock_movements WHERE type = 'recovery' AND product_id = $1`, p).Scan(&ref, &q, &worth))
	if ref == nil || *ref != rec || !q.Equal(decimal.NewFromInt(3)) {
		t.Errorf("the replacement's movement names %v × %s, want recovery %d × 3", ref, q, rec)
	}
	eq(t, "worth back", worth, "36")
	s.must(t, s.db.Get(&newCost, `SELECT cost_price FROM stock_batches WHERE product_id = $1 AND id <> $2 ORDER BY id DESC LIMIT 1`, p, lot))
	eq(t, "new lot's cost", newCost, "12")
	s.must(t, s.db.Get(&price, `SELECT selling_price FROM products WHERE id = $1`, p))
	eq(t, "new price", price, "25")
	if got := s.supBal(t, sup); got != dec(t, bal).Add(decimal.NewFromInt(6)).StringFixed(2) {
		t.Errorf("owed %s after a Rs 6 extra, was %s", got, bal)
	}

	// Into the lot already there (4 left at 10 after this write-off): (4 × 10 + 3 × 12) / 7 = 10.8571.
	mv = s.damage(t, p, lot, "3")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "supplierId": sup, "batchId": lot, "extraPerUnit": "2"}))
	var cost decimal.Decimal
	s.must(t, s.db.Get(&cost, `SELECT cost_price FROM stock_batches WHERE id = $1`, lot))
	eq(t, "blended lot cost", cost, "10.8571")
	eq(t, "lot holds", dec(t, s.lotLeft(t, lot)), "7")

	// Another product's lot is refused.
	mv = s.damage(t, p, lot, "1")
	other := s.ownLot(t, s.prod["plain"])
	s.mustRefuse(t, "into another product's lot", s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "batchId": other}))
	s.audit(t)
}

// REC-3: a credit goes on bills oldest first, then the old debt; the rest waits for
// the next bill; what went where is kept. Any amount; the lot's supplier is offered;
// a disabled supplier takes a credit but no extra charge.
func TestREC3CreditWithTheSupplier(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R3 Supplies", "1000")
	p := s.stocked(t, "R3 Cable", s.unitPcs, sup, nil)
	b1 := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "30"}}, nil)
	b2 := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "20"}}, nil)
	lot, _, _ := s.lotOf(t, b1, p)
	var wo struct{ LossID, SupplierID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "reason": "damage", "batchId": lot}, &wo))
	if wo.SupplierID != sup {
		t.Errorf("the write-off offers supplier %d, want the lot's %d", wo.SupplierID, sup)
	}
	mv := wo.LossID

	var lines []struct {
		ID         int64 `json:"id"`
		SupplierID int64 `json:"supplierId"`
	}
	day := time.Now().Format(time.DateOnly)
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/losses/lines?from="+day+"&to="+day, nil, &lines))
	if len(lines) == 0 || lines[0].ID != mv || lines[0].SupplierID != sup {
		t.Errorf("the loss doesn't offer its lot's supplier: %+v", lines)
	}

	// Owed: old debt 1000 + bills 300 + 200 = 1500; a Rs 1,800 credit (far above the
	// Rs 30 loss) clears the bills and the old debt; 300 waits.
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "1800"}))
	rec := s.lastRecovery(t)
	eq(t, "bill 1 credited", s.bill(t, b1).Credited, "300")
	eq(t, "bill 2 credited", s.bill(t, b2).Credited, "200")
	if bal, _, old := s.supState(t, sup); bal.StringFixed(2) != "-300.00" || !old.IsZero() {
		t.Errorf("balance %s, old debt %s; want -300, 0", bal, old)
	}
	var onBills, oldDebt, left decimal.Decimal
	s.must(t, s.db.QueryRow(`SELECT COALESCE((SELECT SUM(amount) FROM loss_recovery_bills WHERE recovery_id = lr.id), 0), old_debt, credit_unapplied
		FROM loss_recoveries lr WHERE id = $1`, rec).Scan(&onBills, &oldDebt, &left))
	eq(t, "kept on bills", onBills, "500")
	eq(t, "kept on old debt", oldDebt, "1000")
	eq(t, "left for the next bill", left, "300")
	b3 := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "5", "costPrice": "40"}}, nil)
	eq(t, "next bill settled from it", s.bill(t, b3).Credited, "200")

	// Disabled: a credit goes through, an extra charge is refused.
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), nil, nil))
	mv = s.damage(t, p, lot, "1")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "10"}))
	mv = s.damage(t, p, lot, "1")
	s.mustRefuse(t, "an extra charge for a disabled supplier", s.recover(map[string]any{"sourceType": "damage", "sourceId": mv,
		"outcome": "replacement", "supplierId": sup, "batchId": lot, "extraPerUnit": "1"}))
	s.audit(t)
}

// REC-4: written off moves nothing and leaves the claims list.
func TestREC4WrittenOff(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R4 Supplies", "")
	p := s.stocked(t, "R4 Jar", s.unitPcs, sup, nil)
	id := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "5", "costPrice": "10"}}, nil)
	lot, _, _ := s.lotOf(t, id, p)
	mv := s.damage(t, p, lot, "2")
	before, bal := s.onHand(t, p), s.supBal(t, sup)
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "written_off"}))
	if s.onHand(t, p) != before || s.supBal(t, sup) != bal {
		t.Errorf("written off moved stock or the balance")
	}
	if w := s.waiting(t, sup); len(w) != 0 {
		t.Errorf("a written-off loss still waits: %v", w)
	}
	s.audit(t)
}

// waiting is a supplier's claims list's waiting losses (movement ids).
func (s *shop) waiting(t *testing.T, sup int64) []int64 {
	t.Helper()
	var r struct {
		Waiting []struct{ ID int64 } `json:"waiting"`
	}
	s.must(t, s.admin.Do("GET", supPath(sup, "claims"), nil, &r))
	var out []int64
	for _, w := range r.Waiting {
		out = append(out, w.ID)
	}
	return out
}

// REC-6: a claims list shows only that supplier's own, and only what's unsettled.
func TestREC6ClaimsListIsTheirOwn(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	a, b := s.newSup(t, "R6 A", ""), s.newSup(t, "R6 B", "")
	p := s.stocked(t, "R6 Tin", s.unitPcs, 0, nil)
	la, _, _ := s.lotOf(t, s.mustDeliver(t, a, []sl{{"productId": p, "quantity": "5", "costPrice": "10"}}, nil), p)
	lb, _, _ := s.lotOf(t, s.mustDeliver(t, b, []sl{{"productId": p, "quantity": "5", "costPrice": "10"}}, nil), p)
	mv := s.damage(t, p, la, "1")
	if w := s.waiting(t, a); len(w) != 1 || w[0] != mv {
		t.Errorf("A's list %v, want [%d]", w, mv)
	}
	if w := s.waiting(t, b); len(w) != 0 {
		t.Errorf("B's list shows A's loss: %v", w)
	}
	s.mustRefuse(t, "give B's lot back through A", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/give-back", a, lb),
		map[string]string{"counted": "5"}, nil))
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "written_off"}))
	if w := s.waiting(t, a); len(w) != 0 {
		t.Errorf("a settled loss still listed: %v", w)
	}

	// A recalled lot is listed as recalled, and only on its own supplier's list.
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/%d/recall", b, lb), map[string]bool{"stopSelling": false}, nil))
	var cl struct {
		Recalled []struct{ LotID int64 } `json:"recalled"`
	}
	s.must(t, s.admin.Do("GET", supPath(b, "claims"), nil, &cl))
	if len(cl.Recalled) != 1 || cl.Recalled[0].LotID != lb {
		t.Errorf("B's recalled lots %+v, want [%d]", cl.Recalled, lb)
	}
	s.must(t, s.admin.Do("GET", supPath(a, "claims"), nil, &cl))
	if len(cl.Recalled) != 0 {
		t.Errorf("A's list shows B's recalled lot")
	}

	// A write-off of their lot waits 180 days; one given back to them waits until settled.
	old, gone := s.damage(t, p, la, "1"), s.damage(t, p, la, "1")
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE stock_movements SET created_at = NOW() - interval '179 days' WHERE id = %d;
		UPDATE stock_movements SET created_at = NOW() - interval '181 days' WHERE id = %d`, old, gone)))
	if w := s.waiting(t, a); len(w) != 1 || w[0] != old {
		t.Errorf("A's list %v, want only the 179-day-old write-off %d", w, old)
	}
	s.must(t, sqlxExec(t.Context(), s.db, fmt.Sprintf(`UPDATE stock_movements SET given_back_to = %d WHERE id = %d`, a, gone)))
	if w := s.waiting(t, a); len(w) != 2 {
		t.Errorf("A's list %v, want the given-back one too, however old", w)
	}
	s.audit(t)
}

// REC-8: a recovery is undone exactly while nothing came after it.
func TestREC8Undo(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "R8 Supplies", "")
	p := s.stocked(t, "R8 Fan", s.unitPcs, sup, nil)
	id := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "10"}}, nil)
	lot, _, _ := s.lotOf(t, id, p)
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "100", "method": "discount"}, nil))

	// A top-up with an extra charge, undone: stock, lot cost and the balance as before.
	mv := s.damage(t, p, lot, "2")
	onHand, bal := s.onHand(t, p), s.supBal(t, sup)
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "supplierId": sup, "batchId": lot, "extraPerUnit": "3"}))
	s.must(t, s.undo(s.lastRecovery(t)))
	var cost decimal.Decimal
	s.must(t, s.db.Get(&cost, `SELECT cost_price FROM stock_batches WHERE id = $1`, lot))
	if s.onHand(t, p) != onHand || s.supBal(t, sup) != bal || !cost.Equal(decimal.NewFromInt(10)) || s.lotLeft(t, lot) != "8.000" {
		t.Errorf("after undo: on hand %s (was %s), owed %s (was %s), lot cost %s holds %s", s.onHand(t, p), onHand, s.supBal(t, sup), bal, cost, s.lotLeft(t, lot))
	}
	if w := s.waiting(t, sup); len(w) != 1 || w[0] != mv {
		t.Errorf("the loss doesn't wait again: %v", w)
	}

	// Into a new lot, then one sold: undo refused.
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "replacement", "expiry": "2020-01-01"}))
	rec := s.lastRecovery(t)
	var newLot int64
	s.must(t, s.db.Get(&newLot, `SELECT id FROM stock_batches WHERE product_id = $1 AND id <> $2`, p, lot))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": p, "quantity": "1", "batchId": newLot, "reason": "damage"}, nil))
	s.mustRefuse(t, "undo after one of its units left", s.undo(rec))

	// A credit, undone while unspent; refused once a bill came after.
	mv = s.damage(t, p, lot, "1")
	bal = s.supBal(t, sup)
	b := s.bill(t, id)
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "50"}))
	s.must(t, s.undo(s.lastRecovery(t)))
	if s.supBal(t, sup) != bal || !s.bill(t, id).Credited.Equal(b.Credited) {
		t.Errorf("credit undone: owed %s (was %s), bill credited %s (was %s)", s.supBal(t, sup), bal, s.bill(t, id).Credited, b.Credited)
	}
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "paid", "supplierId": sup, "amount": "50"}))
	rec = s.lastRecovery(t)
	s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "1", "costPrice": "10"}}, nil)
	s.mustRefuse(t, "undo a credit after a bill came", s.undo(rec))

	// Written off: always.
	mv = s.damage(t, p, lot, "1")
	s.must(t, s.recover(map[string]any{"sourceType": "damage", "sourceId": mv, "outcome": "written_off"}))
	s.must(t, s.undo(s.lastRecovery(t)))
	if n := s.count(t, `SELECT count(*) FROM loss_recoveries WHERE source_id = $1`, mv); n != 0 {
		t.Errorf("written off still recorded")
	}
	s.audit(t)
}
