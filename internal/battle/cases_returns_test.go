package battle

import "testing"

// RET-5: returning every line of a bill-discounted sale, one at a time, refunds
// exactly what was paid — even when the discount doesn't split evenly.
func TestRET5_ReturnEveryLineRefundsWhatWasPaid(t *testing.T) {
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
