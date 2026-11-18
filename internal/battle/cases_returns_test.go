package battle

import (
	"fmt"
	"testing"
)

// RET-5: returning every line of a bill-discounted sale, one at a time, refunds
// exactly what was paid — even when the discount doesn't split evenly.
func TestRET5_ReturnEveryLineRefundsWhatWasPaid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		product string
		lines   int
		qty     string
		billOff string
		paid    string
	}{
		{"3 × Rs 100, Rs 10 off", "plain", 3, "1", "10", "290.00"},
		{"7 × Rs 33.33, Rs 5 off", "odd", 7, "1", "5", "228.31"},
		{"3 × 3 × Rs 0.99, Rs 1 off", "cheap", 3, "3", "1", "7.91"},
	}
	s := newShop(t)
	s.open(t, s.till1)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items := make([]line, c.lines)
			for i := range items {
				items[i] = line{"productId": s.prod[c.product], "quantity": c.qty}
			}
			r := s.sell(t, s.till1, c.billOff, items, cash("100000"))
			if r.Total != c.paid {
				t.Fatalf("total %s, want %s", r.Total, c.paid)
			}
			refunded := dec(t, "0")
			for _, l := range s.sale(t, s.till1, r.ID).Lines {
				refunded = refunded.Add(s.giveBack(t, s.till1, r.ID, l.ID, l.Quantity))
			}
			if !refunded.Equal(dec(t, c.paid)) {
				t.Errorf("refunded %s, paid %s", refunded.StringFixed(2), c.paid)
			}
			if st := s.sale(t, s.till1, r.ID).Status; st != "returned" {
				t.Errorf("status %s, want returned", st)
			}
		})
	}
	s.audit(t)
}

// RET-4: a line returned one unit at a time refunds exactly its paid value.
func TestRET4_LineInPartsRefundsItsPaidValue(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "1", []line{{"productId": s.prod["plain"], "quantity": "3"}, {"productId": s.prod["odd"], "quantity": "1"}}, cash("1000"))
	v := s.sale(t, s.till1, r.ID)
	want := dec(t, v.Lines[0].Paid)
	got := dec(t, "0")
	for range 3 {
		got = got.Add(s.giveBack(t, s.till1, r.ID, v.Lines[0].ID, "1"))
	}
	if !got.Equal(want) {
		t.Errorf("refunded %s in parts, line paid %s", got.StringFixed(2), want.StringFixed(2))
	}
	s.audit(t)
}

// RET-10: a returned serial-tracked unit can be sold again, and its warranty
// ends with the return.
func TestRET10_ReturnedSerialSellsAgain(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["serial"], "newQuantity": "1", "note": "one phone"}, nil))
	s.open(t, s.till1)
	phone := line{"productId": s.prod["serial"], "quantity": "1", "serials": []string{"IMEI-0001"}}
	first := s.sell(t, s.till1, "0", []line{phone}, cash("15000"))
	s.giveBack(t, s.till1, first.ID, s.sale(t, s.till1, first.ID).Lines[0].ID, "1")

	var r saleResult
	if err := s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"items": []line{phone}, "payments": []pay{cash("15000")}}, &r); err != nil {
		t.Fatalf("selling the returned phone again: %v", err)
	}
	var live int
	s.must(t, s.db.Get(&live, `SELECT count(*) FROM warranty_units WHERE serial_no = 'IMEI-0001' AND sale_id = $1 AND status = 'active'`, first.ID))
	if live != 0 {
		t.Errorf("the returned sale still holds the phone's warranty")
	}
	s.audit(t)
}

// RET-19: a credit sale the customer has already paid off is refunded in cash
// when it comes back; it never turns into credit with the shop.
func TestRET19_PaidOffCreditSaleRefundsCash(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	var r saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": s.customer, "items": []line{{"productId": s.prod["plain"], "quantity": "10"}},
		"payments": []pay{{"method": "credit", "amount": "1000"}}}, &r))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", s.customer), map[string]any{"amount": "1000", "method": "cash"}, nil))

	if v := s.sale(t, s.till1, r.ID); v.OwedOnSale != "0.00" {
		t.Errorf("return screen says %s still owed on the sale, want 0.00 (TILL-2)", v.OwedOnSale)
	}
	cashBack := s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "10")
	if cashBack.StringFixed(2) != "1000.00" {
		t.Errorf("cash refunded %s, want 1000.00", cashBack.StringFixed(2))
	}
	var bal string
	s.must(t, s.db.Get(&bal, `SELECT outstanding_balance FROM customers WHERE id = $1`, s.customer))
	if bal != "0.00" {
		t.Errorf("customer balance %s, want 0.00 (no credit with the shop)", bal)
	}
	s.audit(t)
}

// RET-18 / RET-19: still owing part of it, the return clears what they owe and
// the rest is cash.
func TestRET19_PartPaidCreditSaleClearsDebtThenCash(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	var r saleResult
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": s.customer, "items": []line{{"productId": s.prod["plain"], "quantity": "10"}},
		"payments": []pay{{"method": "credit", "amount": "1000"}}}, &r))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", s.customer), map[string]any{"amount": "600", "method": "cash"}, nil))

	if v := s.sale(t, s.till1, r.ID); v.OwedOnSale != "400.00" {
		t.Errorf("return screen says %s still owed on the sale, want 400.00 (TILL-2)", v.OwedOnSale)
	}
	cashBack := s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "10")
	if cashBack.StringFixed(2) != "600.00" {
		t.Errorf("cash refunded %s, want 600.00 (400 cleared the debt)", cashBack.StringFixed(2))
	}
	var bal string
	s.must(t, s.db.Get(&bal, `SELECT outstanding_balance FROM customers WHERE id = $1`, s.customer))
	if bal != "0.00" {
		t.Errorf("customer balance %s, want 0.00", bal)
	}
	s.audit(t)
}

// RET-13: a combo comes back whole — every line, in whole sets — or not at all.
func TestRET13_ComboComesBackWholeSets(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	combo := map[string]any{"no": 1, "label": "Combo", "sets": "2"}
	r := s.sell(t, s.till1, "0", []line{
		{"productId": s.prod["plain"], "quantity": "2", "group": combo},
		{"productId": s.prod["odd"], "quantity": "4", "group": combo},
		{"productId": s.prod["cheap"], "quantity": "1"},
	}, cash("1000"))
	v := s.sale(t, s.till1, r.ID)
	plain, odd, cheap := v.Lines[0].ID, v.Lines[1].ID, v.Lines[2].ID
	back := func(lines ...map[string]any) error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": lines}, nil)
	}
	ln := func(id int64, q string) map[string]any { return map[string]any{"saleItemId": id, "quantity": q} }

	if err := back(ln(plain, "2")); err == nil {
		t.Error("one line of a combo came back on its own")
	}
	if err := back(ln(plain, "1"), ln(odd, "3")); err == nil {
		t.Error("a combo came back in uneven parts (1 of 2 sets, 3 of 4)")
	}
	if err := back(ln(cheap, "1")); err != nil {
		t.Errorf("a line outside the combo: %v", err)
	}
	if err := back(ln(plain, "1"), ln(odd, "2")); err != nil {
		t.Errorf("one whole set: %v", err)
	}
	if err := back(ln(plain, "1"), ln(odd, "2")); err != nil {
		t.Errorf("the other whole set: %v", err)
	}
	if st := s.sale(t, s.till1, r.ID).Status; st != "returned" {
		t.Errorf("status %s, want returned", st)
	}
	s.audit(t)
}

// RET-4 / RET-7: a loose line returned in many small parts never refunds more
// than was paid for it, at any point (each part used to round on its own: 1 kg at
// Rs 245.50 back a gram at a time refunded Rs 0.25 a gram, past the Rs 245.50 paid).
func TestRET4_ManySmallPartsNeverPassWhatWasPaid(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["loose"], "quantity": "1"}}, cash("300"))
	v := s.sale(t, s.till1, r.ID)
	paid := dec(t, v.Lines[0].Paid)
	got := dec(t, "0")
	for i := 0; i < 990; i++ {
		got = got.Add(s.giveBack(t, s.till1, r.ID, v.Lines[0].ID, "0.001"))
		if got.GreaterThan(paid) {
			t.Fatalf("after %d g back: refunded %s, paid %s", i+1, got.StringFixed(2), paid.StringFixed(2))
		}
	}
	got = got.Add(s.giveBack(t, s.till1, r.ID, v.Lines[0].ID, "0.010"))
	if !got.Equal(paid) {
		t.Errorf("all back: refunded %s, paid %s", got.StringFixed(2), paid.StringFixed(2))
	}
}
