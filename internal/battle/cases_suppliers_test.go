package battle

import (
	"context"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/suppliers"
)

// Area 5 cases, one per rule (docs/rules/purchasing.md): goods sent back,
// the supplier's balance and money.

func supPath(id int64, what string) string {
	return fmt.Sprintf("/api/v1/admin/suppliers/%d/%s", id, what)
}

// sendBack sends goods back (purchase 0 = against no delivery).
func (s *shop) sendBack(t *testing.T, sup, purchase int64, lines ...map[string]any) error {
	t.Helper()
	body := map[string]any{"supplierId": sup, "lines": lines}
	if purchase > 0 {
		body["purchaseId"] = purchase
	}
	return s.admin.Do("POST", "/api/v1/admin/purchase-returns", body, nil)
}

// ownLot is a product's lot that came on no delivery (the shop's own count).
func (s *shop) ownLot(t *testing.T, p int64) int64 {
	t.Helper()
	var id int64
	s.must(t, s.db.Get(&id, `SELECT id FROM stock_batches WHERE product_id = $1 AND purchase_item_id IS NULL ORDER BY id LIMIT 1`, p))
	return id
}

// PUR-9: goods sent back name their lot, from that delivery (or from none);
// priced at the lot's cost; the delivery's bill first, then the oldest; beyond
// every bill it's the supplier's credit.
func TestPUR9GoodsSentBack(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Returns Ltd", "")
	old := s.mustDeliver(t, sup, []sl{{"productId": s.prod["odd"], "quantity": "10", "costPrice": "30"}}, nil)    // 300
	mine := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "20"}}, nil) // 200
	other := s.mustDeliver(t, sup, []sl{{"productId": s.prod["cheap"], "quantity": "5", "costPrice": "4"}}, nil)  // 20
	lot, _, _ := s.lotOf(t, mine, s.prod["plain"])
	odd, _, _ := s.lotOf(t, old, s.prod["odd"])
	s.mustRefuse(t, "no lot named", s.sendBack(t, sup, mine, map[string]any{"productId": s.prod["plain"], "quantity": "1"}))
	s.mustRefuse(t, "a lot from another delivery", s.sendBack(t, sup, other, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": lot}))
	s.mustRefuse(t, "a delivery's lot sent back against none", s.sendBack(t, sup, 0, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": lot}))
	s.mustRefuse(t, "the shop's own lot against a delivery", s.sendBack(t, sup, mine, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": s.ownLot(t, s.prod["plain"])}))
	s.mustRefuse(t, "more than the lot holds", s.sendBack(t, sup, mine, map[string]any{"productId": s.prod["plain"], "quantity": "11", "batchId": lot}))
	if got := s.supBal(t, sup); got != "520.00" {
		t.Fatalf("refusals moved the balance to %s", got)
	}
	// All 10 at the lot's cost of 20 (the typed 999 is ignored), off its own bill
	// though an older one is open.
	s.must(t, s.sendBack(t, sup, mine, map[string]any{"productId": s.prod["plain"], "quantity": "10", "costPrice": "999", "batchId": lot}))
	eq(t, "own bill credited", s.bill(t, mine).Credited, "200")
	eq(t, "oldest bill credited", s.bill(t, old).Credited, "0")
	s.must(t, s.sendBack(t, sup, old, map[string]any{"productId": s.prod["odd"], "quantity": "10", "batchId": odd}))
	eq(t, "its own bill", s.bill(t, old).Credited, "300")
	if got := s.supBal(t, sup); got != "20.00" {
		t.Errorf("owed %s, want 20.00", got)
	}
	// Beyond every bill: the shop's own stock sent back leaves a credit.
	s.must(t, s.sendBack(t, sup, 0, map[string]any{"productId": s.prod["plain"], "quantity": "50", "batchId": s.ownLot(t, s.prod["plain"])}))
	eq(t, "the rest off the last bill", s.bill(t, other).Credited, "20")
	if got := s.supBal(t, sup); got != "-30.00" {
		t.Errorf("balance %s, want -30.00 (their credit)", got)
	}
	s.audit(t)
}

// PUR-9 (user, Q4): a frozen product's pieces out for sale go back to the
// supplier as a frozen write-off does — put back on the books, then sent back.
func TestPUR9FrozenPiecesSentBack(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Frozen Ltd", "")
	p := s.stocked(t, "Frozen pens", s.unitPcs, 0, nil)
	got := s.mustDeliver(t, sup, []sl{{"productId": p, "quantity": "10", "costPrice": "20"}}, nil)
	lot, _, _ := s.lotOf(t, got, p)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/freeze", map[string]any{"productIds": []int64{p}}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/stock/%d/to-shelf", p), map[string]string{"quantity": "8"}, nil))
	// 6 of the 8 out for sale go back: storage keeps its 2.
	s.must(t, s.sendBack(t, sup, got, map[string]any{"productId": p, "quantity": "6", "batchId": lot, "fromShelf": true}))
	if q := s.onHand(t, p); q != "2.000" {
		t.Errorf("on hand %s, want 2.000 (storage untouched)", q)
	}
	if got := s.supBal(t, sup); got != "80.00" {
		t.Errorf("owed %s, want 80.00", got)
	}
	s.audit(t)
}

// SUP-2: an opening is booked once, never below zero; a sheet leaves an existing
// supplier's alone; adjusting it moves the balance by the same amount; a credit
// they hold pays a newly set old debt down (user, Q2).
func TestSUP2Opening(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "10000"))
	s.mustRefuse(t, "negative opening", s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Minus Ltd", "openingBalance": "-5"}, nil))
	sup := s.newSup(t, "Opening Ltd", "1000")
	s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "5", "costPrice": "100"}}, nil) // 500
	res, err := suppliers.NewService(s.db).ImportOne(context.Background(), suppliers.CreateInput{Name: "Opening Ltd"}, decimal.NewFromInt(9000), nil)
	s.must(t, err)
	if res.Action != "updated" {
		t.Errorf("sheet row for an existing supplier: %s", res.Action)
	}
	bal, opening, oldDebt := s.supState(t, sup)
	eq(t, "balance after the sheet", bal, "1500")
	s.must(t, s.admin.Do("POST", supPath(sup, "opening"), map[string]string{"opening": "700"}, nil))
	bal, opening, oldDebt = s.supState(t, sup)
	eq(t, "balance", bal, "1200")
	eq(t, "opening", opening, "700")
	eq(t, "old debt", oldDebt, "700")
	s.mustRefuse(t, "adjusted below zero", s.admin.Do("POST", supPath(sup, "opening"), map[string]string{"opening": "-100"}, nil))
	eq(t, "old debt after the refusal", func() decimal.Decimal { _, _, o := s.supState(t, sup); return o }(), "700")
	// A supplier holding Rs 300 of ours: old debt set to 1000 is 700 once the credit pays it.
	adv := s.newSup(t, "Advance Ltd", "")
	s.must(t, s.admin.Do("POST", supPath(adv, "payment"), map[string]string{"amount": "300", "method": "cash", "source": safe}, nil))
	s.must(t, s.admin.Do("POST", supPath(adv, "opening"), map[string]string{"opening": "1000"}, nil))
	bal, _, oldDebt = s.supState(t, adv)
	eq(t, "balance", bal, "700")
	eq(t, "old debt paid down by the credit", oldDebt, "700")
	s.audit(t)
}

// SUP-3: cash comes from a till or cash place, card / online from a bank;
// paid from outside moves nothing; the money receipt is the amount.
func TestSUP3MoneyLandsWhereItSays(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "10000")
	bank := s.locker(t, "Bank", "bank", "10000")
	sup := s.newSup(t, "Places Ltd", "5000")
	pay := func(method string, src string) error {
		return s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "100", "method": method, "source": src}, nil)
	}
	s.refused(t, "cash from a bank", func() error { return pay("cash", fmt.Sprintf("locker:%d", bank)) })
	s.refused(t, "card from the safe", func() error { return pay("card", fmt.Sprintf("locker:%d", safe)) })
	s.refused(t, "cash from nowhere", func() error { return pay("cash", "") })
	s.must(t, pay("card", fmt.Sprintf("locker:%d", bank)))
	s.must(t, pay("cash", fmt.Sprintf("locker:%d", safe)))
	before := s.rows(t)
	s.must(t, pay("online", ""))
	if s.rows(t) != before {
		t.Errorf("paid from outside moved money")
	}
	eq(t, "bank", s.lockerBal(t, bank), "9900")
	eq(t, "safe", s.lockerBal(t, safe), "9900")
	if got := s.supBal(t, sup); got != "4700.00" {
		t.Errorf("owed %s, want 4700.00", got)
	}
	s.audit(t)
}

// SUP-4: oldest first, old debt first, or typed per bill; never beyond a bill,
// another supplier's bill, or the old debt — and a refused one saves nothing.
func TestSUP4WherePaymentsGo(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	src := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "100000"))
	sup := s.newSup(t, "Queue Ltd", "500")
	b1 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "300"}}, nil)
	b2 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "400"}}, nil)
	pay := func(body map[string]any) error {
		body["method"], body["source"] = "cash", src
		return s.admin.Do("POST", supPath(sup, "payment"), body, nil)
	}
	s.must(t, pay(map[string]any{"amount": "350"})) // oldest first: b1 300, b2 50
	eq(t, "b1", s.bill(t, b1).Paid, "300")
	eq(t, "b2", s.bill(t, b2).Paid, "50")
	s.must(t, pay(map[string]any{"amount": "200", "mode": "old"})) // old debt first
	_, _, oldDebt := s.supState(t, sup)
	eq(t, "old debt", oldDebt, "300")
	s.must(t, pay(map[string]any{"allocations": []map[string]any{{"purchaseId": b2, "amount": "100"}}}))
	eq(t, "typed on b2", s.bill(t, b2).Paid, "150")
	other := s.newSup(t, "Other Ltd", "")
	ob := s.mustDeliver(t, other, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "100"}}, nil)
	rows := s.rows(t)
	s.mustRefuse(t, "beyond a bill", pay(map[string]any{"allocations": []map[string]any{{"purchaseId": b2, "amount": "251"}}}))
	s.mustRefuse(t, "another supplier's bill", pay(map[string]any{"allocations": []map[string]any{{"purchaseId": ob, "amount": "10"}}}))
	s.mustRefuse(t, "beyond the old debt", pay(map[string]any{"oldDebt": "301"}))
	if s.rows(t) != rows {
		t.Errorf("a refused payment moved money")
	}
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM supplier_payments WHERE supplier_id = $1`, sup))
	if n != 3 {
		t.Errorf("%d payments saved, want 3", n)
	}
	if got := s.supBal(t, sup); got != "550.00" {
		t.Errorf("owed %s, want 550.00", got)
	}
	s.audit(t)
}

// SUP-5 (user, Q1): an advance typed while bills are open pays them first,
// oldest first, then the old debt; only the rest stays as credit.
func TestSUP5TypedAdvancePaysWhatsOwedFirst(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	src := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "100000"))
	sup := s.newSup(t, "Ahead Ltd", "100")
	b := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "300"}}, nil)
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"advance": "1000", "method": "cash", "source": src}, nil))
	if bb := s.bill(t, b); bb.Status != "paid" {
		t.Errorf("open bill %s after an advance, want paid", bb.Status)
	}
	bal, _, oldDebt := s.supState(t, sup)
	eq(t, "old debt", oldDebt, "0")
	eq(t, "balance (their credit)", bal, "-600")
	// Beyond everything: the next bill is settled from what's left.
	nb := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "250"}}, nil)
	eq(t, "next bill paid from the credit", s.bill(t, nb).Paid, "250")
	s.audit(t)
}

// SUP-6: a discount lowers bills (credited) and the balance, moves no money,
// and never passes what's owed.
func TestSUP6DiscountReceived(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Kind Ltd", "")
	b := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "300"}}, nil)
	rows := s.rows(t)
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "40", "method": "discount"}, nil))
	bb := s.bill(t, b)
	eq(t, "credited", bb.Credited, "40")
	eq(t, "paid", bb.Paid, "0")
	s.mustRefuse(t, "discount beyond what's owed", s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "261", "method": "discount"}, nil))
	if s.rows(t) != rows {
		t.Errorf("a discount moved money")
	}
	if got := s.supBal(t, sup); got != "260.00" {
		t.Errorf("owed %s, want 260.00", got)
	}
	s.audit(t)
}

// SUP-7: a refund is at most the credit held, lands in the right place, and
// walks the balance back toward zero.
func TestSUP7RefundReceived(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "10000")
	bank := s.locker(t, "Bank", "bank", "0")
	sup := s.newSup(t, "Owes Us Ltd", "")
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "500", "method": "cash", "source": fmt.Sprintf("locker:%d", safe)}, nil))
	refund := func(amount, method string, place int64) error {
		return s.admin.Do("POST", supPath(sup, "refund"), map[string]string{"amount": amount, "method": method, "source": fmt.Sprintf("locker:%d", place)}, nil)
	}
	s.refused(t, "refund above the credit", func() error { return refund("501", "cash", safe) })
	s.refused(t, "card into the safe", func() error { return refund("100", "card", safe) })
	s.must(t, refund("200", "card", bank))
	s.must(t, refund("300", "cash", safe))
	eq(t, "bank", s.lockerBal(t, bank), "200")
	eq(t, "safe", s.lockerBal(t, safe), "9800")
	if got := s.supBal(t, sup); got != "0.00" {
		t.Errorf("balance %s, want 0.00", got)
	}
	s.refused(t, "refund with no credit", func() error { return refund("1", "cash", safe) })
	s.audit(t)
}

// SUP-9 (user, Q3): a disabled supplier keeps their balance and can be paid,
// but nothing is delivered, ordered or sent back for them.
func TestSUP9DisabledSupplier(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	src := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "10000"))
	sup := s.newSup(t, "Gone Ltd", "")
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}, nil)
	lot, _, _ := s.lotOf(t, got, s.prod["plain"])
	var o struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup, "lines": []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50"}}}, &o))
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), nil, nil))
	_, err := s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50"}}, nil)
	s.mustRefuse(t, "delivery from a disabled supplier", err)
	s.mustRefuse(t, "order from a disabled supplier", s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup,
		"lines": []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50"}}}, nil))
	s.mustRefuse(t, "their order received", s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/purchases/%d/receive", o.ID), map[string]any{"supplierId": sup,
		"lines": []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50"}}}, nil))
	s.mustRefuse(t, "goods sent back to them", s.sendBack(t, sup, got, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": lot}))
	if b := s.supBal(t, sup); b != "500.00" {
		t.Errorf("balance %s, want 500.00 kept", b)
	}
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "100", "method": "cash", "source": src}, nil))
	if b := s.supBal(t, sup); b != "400.00" {
		t.Errorf("balance %s after paying a disabled supplier, want 400.00", b)
	}
	s.audit(t)
}

// SUP-10: the statement's running balance ends at the supplier's balance; Owing
// lists every supplier owed — a disabled one too — with the oldest bill unpaid.
func TestSUP10StatementAndOwing(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	src := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "100000"))
	sup := s.newSup(t, "Ledger Ltd", "250")
	first := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "4", "costPrice": "75"}}, nil)
	s.mustDeliver(t, sup, []sl{{"productId": s.prod["odd"], "quantity": "3", "costPrice": "20"}}, nil)
	lot, _, _ := s.lotOf(t, first, s.prod["plain"])
	s.must(t, s.sendBack(t, sup, first, map[string]any{"productId": s.prod["plain"], "quantity": "1", "batchId": lot}))
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "120", "method": "cash", "source": src}, nil))
	s.must(t, s.admin.Do("POST", supPath(sup, "payment"), map[string]string{"amount": "10", "method": "discount"}, nil))
	s.must(t, s.admin.Do("POST", supPath(sup, "opening"), map[string]string{"opening": "200"}, nil))
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), nil, nil))
	var st struct{ Balance string }
	s.must(t, s.admin.Do("GET", supPath(sup, "statement"), nil, &st))
	if want := s.supBal(t, sup); st.Balance != want {
		t.Errorf("statement ends at %s, balance %s", st.Balance, want)
	}
	rows, err := suppliers.NewService(s.db).Owing(context.Background())
	s.must(t, err)
	found := false
	for _, r := range rows {
		if r.ID == sup {
			found = true
			if r.OldestUnpaid == nil {
				t.Errorf("Owing: no oldest unpaid bill for a supplier with bills open")
			}
		}
	}
	if !found {
		t.Errorf("Owing leaves out a disabled supplier who is owed")
	}
	s.audit(t)
}

// PUR-9: the shop's own lot whose cost box was left blank goes back at the
// product's cost (as a write-off does, STK-7), not Rs 0.
func TestPUR9BlankCostOwnLotAtProductCost(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Blank Back Ltd", "")
	p := s.stocked(t, "Blank-cost torches", s.unitPcs, 0, nil) // product cost 10
	s.addLot(t, p, "5", "10", "")
	lot := s.lots(t, p)[0].ID
	s.db.MustExec(`UPDATE stock_batches SET cost_price = 0 WHERE id = $1`, lot)
	s.must(t, s.sendBack(t, sup, 0, map[string]any{"productId": p, "quantity": "2", "batchId": lot}))
	var total decimal.Decimal
	s.must(t, s.db.Get(&total, `SELECT total FROM purchase_returns WHERE supplier_id = $1`, sup))
	eq(t, "debit note for 2 blank-cost units", total, "20")
	if got := s.supBal(t, sup); got != "-20.00" {
		t.Errorf("supplier balance %s, want -20.00 (the shop's credit)", got)
	}
	s.audit(t)
}

// PUR-10: a delivery where nothing cost anything (all free goods) sent back
// whole credits nothing.
func TestPUR10AllFreeDeliveryBackCreditsNothing(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Free Goods Ltd", "")
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "4", "costPrice": "0"}}, nil)
	lot, _, _ := s.lotOf(t, got, s.prod["plain"])
	s.must(t, s.sendBack(t, sup, got, map[string]any{"productId": s.prod["plain"], "quantity": "4", "batchId": lot}))
	var total decimal.Decimal
	s.must(t, s.db.Get(&total, `SELECT total FROM purchase_returns WHERE supplier_id = $1`, sup))
	eq(t, "debit note for free goods", total, "0")
	if b := s.supBal(t, sup); b != "0.00" {
		t.Errorf("supplier balance %s, want 0.00", b)
	}
	s.audit(t)
}
