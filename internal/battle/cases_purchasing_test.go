package battle

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
)

// Area 5 cases, one per rule (docs/rules/purchasing.md): bills and deliveries.

type sl = map[string]any // a StockLine on a delivery / order

// newSup makes a supplier (opening "" = none).
func (s *shop) newSup(t *testing.T, name, opening string) int64 {
	t.Helper()
	body := map[string]string{"name": name}
	if opening != "" {
		body["openingBalance"] = opening
	}
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", body, &sup))
	return sup.ID
}

// deliver books a delivery with no order; extra adds discount / pay / invoiceNo.
func (s *shop) deliver(t *testing.T, sup int64, lines []sl, extra map[string]any) (int64, error) {
	t.Helper()
	body := map[string]any{"supplierId": sup, "lines": lines}
	for k, v := range extra {
		body[k] = v
	}
	var r struct{ PurchaseID int64 }
	err := s.admin.Do("POST", "/api/v1/admin/purchases/receive", body, &r)
	return r.PurchaseID, err
}

func (s *shop) mustDeliver(t *testing.T, sup int64, lines []sl, extra map[string]any) int64 {
	t.Helper()
	id, err := s.deliver(t, sup, lines, extra)
	s.must(t, err)
	return id
}

type billRow struct {
	Subtotal, Discount, Total, Paid, Credited decimal.Decimal
	Status                                    string
}

func (s *shop) bill(t *testing.T, id int64) billRow {
	t.Helper()
	var b billRow
	s.must(t, s.db.QueryRow(`SELECT subtotal, discount, total, paid_amount, credited_amount, status FROM purchases WHERE id = $1`, id).
		Scan(&b.Subtotal, &b.Discount, &b.Total, &b.Paid, &b.Credited, &b.Status))
	return b
}

// supState is a supplier's balance, opening and old debt.
func (s *shop) supState(t *testing.T, id int64) (bal, opening, old decimal.Decimal) {
	t.Helper()
	s.must(t, s.db.QueryRow(`SELECT outstanding_balance, opening_balance, opening_unlinked FROM suppliers WHERE id = $1`, id).Scan(&bal, &opening, &old))
	return
}

func (s *shop) supBal(t *testing.T, id int64) string {
	t.Helper()
	b, _, _ := s.supState(t, id)
	return b.StringFixed(2)
}

// lotOf is the lot a delivery's line for product p opened.
func (s *shop) lotOf(t *testing.T, purchase, p int64) (id int64, qty, cost decimal.Decimal) {
	t.Helper()
	s.must(t, s.db.QueryRow(`SELECT b.id, b.qty_received, b.cost_price FROM stock_batches b JOIN purchase_items pi ON pi.id = b.purchase_item_id
		WHERE pi.purchase_id = $1 AND b.product_id = $2`, purchase, p).Scan(&id, &qty, &cost))
	return
}

func (s *shop) lockerBal(t *testing.T, id int64) decimal.Decimal {
	t.Helper()
	var b decimal.Decimal
	s.must(t, s.db.Get(&b, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, id))
	return b
}

func eq(t *testing.T, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s = %s, want %s", what, got.String(), want)
	}
}

// PUR-1: a bill's total is its lines' nets less the bill discount; line
// discounts (fixed per unit, percent off the line) and the bill discount are
// clamped to what they discount; free units add nothing.
func TestPUR1BillTotal(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Totals Ltd", "")
	id := s.mustDeliver(t, sup, []sl{
		{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50", "discount": "3", "discountType": "fixed"}, // 500 − 30
		{"productId": s.prod["odd"], "quantity": "4", "costPrice": "25", "discount": "10", "discountType": "percent"}, // 100 − 10
		{"productId": s.prod["cheap"], "quantity": "2", "costPrice": "10", "discount": "20", "discountType": "fixed"}, // 20 − 20 (clamped)
		{"productId": s.prod["loose"], "quantity": "3", "freeQty": "5", "costPrice": "100"},                           // 300, free owe nothing
	}, map[string]any{"discount": "10", "discountType": "percent"})
	b := s.bill(t, id)
	eq(t, "subtotal", b.Subtotal, "860")
	eq(t, "bill discount", b.Discount, "86")
	eq(t, "total", b.Total, "774")
	if got := s.supBal(t, sup); got != "774.00" {
		t.Errorf("owed %s, want 774.00", got)
	}
	// A bill discount and a percent line discount beyond what they discount stop at it.
	id2 := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50", "discount": "150", "discountType": "percent"},
		{"productId": s.prod["odd"], "quantity": "1", "costPrice": "40"}}, map[string]any{"discount": "5000", "discountType": "fixed"})
	b2 := s.bill(t, id2)
	eq(t, "clamped subtotal", b2.Subtotal, "40")
	eq(t, "clamped total", b2.Total, "0")
	if got := s.supBal(t, sup); got != "774.00" {
		t.Errorf("owed %s after a nothing-owed bill, want 774.00", got)
	}
	s.audit(t)
}

// PUR-2: each line opens one lot of paid + free units with one purchase
// movement; cost and price change only to a figure above zero; the usual
// supplier is filled in only when the product has none.
func TestPUR2DeliveryLands(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	a := s.newSup(t, "First Ltd", "")
	b := s.newSup(t, "Second Ltd", "")
	owned := s.stocked(t, "Owned", s.unitPcs, a, nil)   // usual supplier a
	orphan := s.stocked(t, "Orphan", s.unitPcs, 0, nil) // none yet
	id := s.mustDeliver(t, b, []sl{
		{"productId": owned, "quantity": "6", "freeQty": "2", "costPrice": "12", "sellingPrice": "25"},
		{"productId": orphan, "quantity": "3", "costPrice": "0", "sellingPrice": "0"},
	}, nil)
	_, qty, _ := s.lotOf(t, id, owned)
	eq(t, "lot of paid + free", qty, "8")
	if typ, q, _ := s.lastMove(t, owned); typ != "purchase" || !q.Equal(decimal.NewFromInt(8)) {
		t.Errorf("movement %s %s, want purchase 8", typ, q)
	}
	var p struct {
		Cost, Sell decimal.Decimal
		Usual      *int64
	}
	read := func(id int64) {
		s.must(t, s.db.QueryRow(`SELECT cost_price, selling_price, preferred_supplier_id FROM products WHERE id = $1`, id).Scan(&p.Cost, &p.Sell, &p.Usual))
	}
	read(owned)
	eq(t, "owned cost", p.Cost, "12")
	eq(t, "owned price", p.Sell, "25")
	if p.Usual == nil || *p.Usual != a {
		t.Errorf("owned's usual supplier changed to %v, want %d", p.Usual, a)
	}
	read(orphan)
	eq(t, "free line leaves cost", p.Cost, "10")
	eq(t, "free line leaves price", p.Sell, "20")
	if p.Usual == nil || *p.Usual != b {
		t.Errorf("orphan's usual supplier %v, want %d", p.Usual, b)
	}
	s.audit(t)
}

// PUR-3: lots are worth what was really paid (line + bill discounts, free units)
// to within a cent a line; sending the whole delivery back clears its bill
// exactly; a line nothing was paid for takes its list cost.
func TestPUR3LotCostIsWhatWasPaid(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Odd Cents Ltd", "")
	lines := []sl{
		{"productId": s.prod["plain"], "quantity": "7", "freeQty": "2", "costPrice": "13.33", "discount": "5", "discountType": "percent"},
		{"productId": s.prod["odd"], "quantity": "3", "costPrice": "9.99"},
		{"productId": s.prod["cheap"], "quantity": "11", "costPrice": "3.17", "discount": "0.05", "discountType": "fixed"},
	}
	id := s.mustDeliver(t, sup, lines, map[string]any{"discount": "7.77", "discountType": "fixed"})
	total := s.bill(t, id).Total
	worth := decimal.Zero
	var back []map[string]any
	for _, l := range lines {
		p := l["productId"].(int64)
		lot, qty, cost := s.lotOf(t, id, p)
		worth = worth.Add(qty.Mul(cost))
		back = append(back, map[string]any{"productId": p, "quantity": qty.String(), "batchId": lot})
	}
	if !near(worth, total, "0.03") {
		t.Errorf("lots worth %s, bill total %s: more than a cent a line apart", worth, total)
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup, "purchaseId": id, "lines": back}, nil))
	b := s.bill(t, id)
	if b.Status != "paid" || !b.Total.Sub(b.Paid).Sub(b.Credited).IsZero() {
		t.Errorf("whole delivery sent back: bill %s with %s left, want paid and nothing left", b.Status, b.Total.Sub(b.Paid).Sub(b.Credited))
	}
	if got := s.supBal(t, sup); got != "0.00" {
		t.Errorf("owed %s after sending it all back, want 0.00 (Q5: the last units take what's left of their share)", got)
	}
	// The last line cost nothing: the line before it takes what's left of the
	// bill, and sending it all back still clears it exactly.
	two := s.mustDeliver(t, sup, []sl{{"productId": s.prod["odd"], "quantity": "7", "costPrice": "3.33"},
		{"productId": s.prod["plain"], "quantity": "13", "costPrice": "7.77"},
		{"productId": s.prod["cheap"], "quantity": "1", "costPrice": "5", "discount": "100", "discountType": "percent"}},
		map[string]any{"discount": "3", "discountType": "percent"})
	for _, p := range []int64{s.prod["odd"], s.prod["plain"], s.prod["cheap"]} {
		lot, qty, _ := s.lotOf(t, two, p)
		s.must(t, s.sendBack(t, sup, two, map[string]any{"productId": p, "quantity": qty.String(), "batchId": lot}))
	}
	if got := s.supBal(t, sup); got != "0.00" {
		t.Errorf("owed %s after sending back a bill whose last line cost nothing, want 0.00", got)
	}
	// Nothing paid on the line (fully discounted): the lot keeps the list cost.
	free := s.mustDeliver(t, sup, []sl{{"productId": s.prod["odd"], "quantity": "2", "costPrice": "8", "discount": "100", "discountType": "percent"}}, nil)
	_, _, cost := s.lotOf(t, free, s.prod["odd"])
	eq(t, "nothing-paid lot cost", cost, "8")
	s.audit(t)
}

// PUR-4: a pack line is kept in units; the pack size is a whole number above 1;
// a whole-unit product never gets a fraction — on a delivery, an order or goods
// sent back.
func TestPUR4PacksAndWholeUnits(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Packs Ltd", "")
	id := s.mustDeliver(t, sup, []sl{{"productId": s.prod["pack"], "quantity": "2", "costPrice": "120", "packSize": "12"}}, nil)
	lot, qty, cost := s.lotOf(t, id, s.prod["pack"])
	eq(t, "units", qty, "24")
	eq(t, "unit cost", cost, "10")
	eq(t, "bill", s.bill(t, id).Total, "240")
	_, err := s.deliver(t, sup, []sl{{"productId": s.prod["pack"], "quantity": "2", "costPrice": "120", "packSize": "2.5"}}, nil)
	s.mustRefuse(t, "pack of 2.5", err)
	_, err = s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "2.5", "costPrice": "10"}}, nil)
	s.mustRefuse(t, "delivery of 2.5 pieces", err)
	_, err = s.deliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "2", "freeQty": "0.5", "costPrice": "10"}}, nil)
	s.mustRefuse(t, "half a piece free", err)
	s.mustRefuse(t, "order of 2.5 pieces", s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup,
		"lines": []sl{{"productId": s.prod["plain"], "quantity": "2.5", "costPrice": "10"}}}, nil))
	s.mustRefuse(t, "2.5 pieces sent back", s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup, "purchaseId": id,
		"lines": []map[string]any{{"productId": s.prod["pack"], "quantity": "2.5", "batchId": lot}}}, nil))
	// Loose goods take a fraction.
	s.mustDeliver(t, sup, []sl{{"productId": s.prod["loose"], "quantity": "2.5", "costPrice": "10"}}, nil)
	s.audit(t)
}

// PUR-5: an order moves nothing; only an order can be edited or deleted.
func TestPUR5OrderMovesNothing(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Orders Ltd", "")
	before, money := s.onHand(t, s.prod["plain"]), s.rows(t)
	var o struct{ ID int64 }
	order := map[string]any{"supplierId": sup, "lines": []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"}}}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", order, &o))
	order["lines"] = []sl{{"productId": s.prod["plain"], "quantity": "12", "costPrice": "50"}}
	s.must(t, s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/purchases/%d", o.ID), order, nil))
	if s.onHand(t, s.prod["plain"]) != before || s.rows(t) != money || s.supBal(t, sup) != "0.00" {
		t.Errorf("an order moved stock, money or the balance")
	}
	var st struct{ Lines []any }
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/suppliers/%d/statement", sup), nil, &st))
	if len(st.Lines) != 0 {
		t.Errorf("an order is on the statement: %d lines", len(st.Lines))
	}
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/purchases/%d", o.ID), nil, nil))
	got := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "1", "costPrice": "50"}}, nil)
	s.mustRefuse(t, "edit a bill", s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/purchases/%d", got), order, nil))
	s.mustRefuse(t, "delete a bill", s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/purchases/%d", got), nil, nil))
	eq(t, "bill after refusals", s.bill(t, got).Total, "50")
	s.audit(t)
}

// PUR-6: an order received takes what came; the rest — a line short and a line
// that didn't come at all — stays on a new order; it is received once.
func TestPUR6ReceivedOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	sup := s.newSup(t, "Short Ltd", "")
	var o struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases", map[string]any{"supplierId": sup, "lines": []sl{
		{"productId": s.prod["plain"], "quantity": "10", "costPrice": "50"},
		{"productId": s.prod["odd"], "quantity": "5", "costPrice": "20"},
		{"productId": s.prod["cheap"], "quantity": "3", "costPrice": "1"}}}, &o))
	recv := map[string]any{"supplierId": sup, "keepRemainder": true, "lines": []sl{
		{"productId": s.prod["plain"], "ordered": "10", "quantity": "8", "costPrice": "50"},
		{"productId": s.prod["odd"], "ordered": "5", "quantity": "5", "costPrice": "20"}}}
	path := fmt.Sprintf("/api/v1/admin/purchases/%d/receive", o.ID)
	s.must(t, s.admin.Do("POST", path, recv, nil))
	eq(t, "bill", s.bill(t, o.ID).Total, "500")
	var rest []struct {
		Product int64           `db:"product_id"`
		Qty     decimal.Decimal `db:"quantity"`
	}
	s.must(t, s.db.Select(&rest, `SELECT pi.product_id, pi.quantity FROM purchase_items pi JOIN purchases p ON p.id = pi.purchase_id
		WHERE p.status = 'draft' AND p.supplier_id = $1 ORDER BY pi.product_id`, sup))
	want := map[int64]string{s.prod["plain"]: "2", s.prod["cheap"]: "3"}
	if len(rest) != 2 {
		t.Fatalf("kept on order %v, want plain 2 and cheap 3", rest)
	}
	for _, r := range rest {
		if !r.Qty.Equal(decimal.RequireFromString(want[r.Product])) {
			t.Errorf("product %d kept %s, want %s", r.Product, r.Qty, want[r.Product])
		}
	}
	s.mustRefuse(t, "received twice", s.admin.Do("POST", path, recv, nil))
	if got := s.supBal(t, sup); got != "500.00" {
		t.Errorf("owed %s, want 500.00 (booked once)", got)
	}
	s.audit(t)
}

// PUR-7: paying on delivery pays only what's left once the supplier's credit
// settled the bill; only that leaves the safe.
func TestPUR7PayOnDeliveryAfterCredit(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "10000")
	sup := s.newSup(t, "Advance Ltd", "")
	src := fmt.Sprintf("locker:%d", safe)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]string{"amount": "300", "method": "cash", "source": src}, nil))
	var r struct {
		PurchaseID int64
		Paid       string
	}
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": sup,
		"lines": []sl{{"productId": s.prod["plain"], "quantity": "20", "costPrice": "50"}},
		"pay":   map[string]string{"amount": "1000", "method": "cash", "source": src}}, &r))
	if r.Paid != "700.00" {
		t.Errorf("paid on delivery %s, want 700.00", r.Paid)
	}
	eq(t, "safe", s.lockerBal(t, safe), "9000")
	if b := s.bill(t, r.PurchaseID); b.Status != "paid" {
		t.Errorf("bill %s, want paid", b.Status)
	}
	if got := s.supBal(t, sup); got != "0.00" {
		t.Errorf("owed %s, want 0.00", got)
	}
	s.audit(t)
}

// PUR-8: a new bill is settled from the advance first (paid), then goods-sent-
// back credit (credited); an advance already refunded isn't spent again.
func TestPUR8SettledFromCredit(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	safe := fmt.Sprintf("locker:%d", s.locker(t, "Safe", "safe", "10000"))
	sup := s.newSup(t, "Credit Ltd", "")
	pay := fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup)
	s.must(t, s.admin.Do("POST", pay, map[string]string{"amount": "200", "method": "cash", "source": safe}, nil))
	var opening int64 // stock that came on no delivery (the shop's own count)
	s.must(t, s.db.Get(&opening, `SELECT id FROM stock_batches WHERE product_id = $1 AND purchase_item_id IS NULL ORDER BY id LIMIT 1`, s.prod["plain"]))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchase-returns", map[string]any{"supplierId": sup,
		"lines": []map[string]any{{"productId": s.prod["plain"], "quantity": "30", "batchId": opening}}}, nil)) // 30 × 1
	id := s.mustDeliver(t, sup, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "100"}}, nil)
	b := s.bill(t, id)
	eq(t, "paid from the advance", b.Paid, "200")
	eq(t, "credited from goods sent back", b.Credited, "30")
	if got := s.supBal(t, sup); got != "770.00" {
		t.Errorf("owed %s, want 770.00", got)
	}
	// An advance refunded in part: only what's still held settles the next bill.
	other := s.newSup(t, "Refunded Ltd", "")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", other), map[string]string{"amount": "500", "method": "cash", "source": safe}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/refund", other), map[string]string{"amount": "300", "method": "cash", "source": safe}, nil))
	id2 := s.mustDeliver(t, other, []sl{{"productId": s.prod["plain"], "quantity": "10", "costPrice": "100"}}, nil)
	eq(t, "paid from what's still held", s.bill(t, id2).Paid, "200")
	s.audit(t)
}
