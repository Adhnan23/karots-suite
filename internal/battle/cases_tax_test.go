package battle

import "testing"

// SALE-35: a registered shop charges tax on what the customer pays — after the
// item and bill discounts (user left it to us, 2026-11-18).
func TestSALE35_TaxAfterTheBillDiscount(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE settings SET tax_registered = true, tax_reg_no = 'VAT-1'`))
	s.must(t, sqlxExec(t.Context(), s.db, `UPDATE products SET tax_rate = 18 WHERE name IN ('Battle plain', 'Battle odd')`))
	s.open(t, s.till1)
	cases := []struct {
		name, bill string
		items      []line
		want       string
	}{
		// nets 100 + 100, Rs 10 off shared 5 + 5: tax 18% of 95 + 95 = 34.20
		{"Rs 10 off two lines", "10", []line{{"productId": s.prod["plain"], "quantity": "1"}, {"productId": s.prod["plain"], "quantity": "1"}}, "224.20"},
		// 100 + 33.33 = 133.33, Rs 1 off shared 0.75 + 0.25: tax 17.87 + 5.95 = 23.82
		{"uneven share", "1", []line{{"productId": s.prod["plain"], "quantity": "1"}, {"productId": s.prod["odd"], "quantity": "1"}}, "156.15"},
		{"no bill discount", "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, "236.00"},
	}
	for _, c := range cases {
		if r := s.sell(t, s.till1, c.bill, c.items, cash("1000")); r.Total != c.want {
			t.Errorf("%s: total %s, want %s", c.name, r.Total, c.want)
		}
	}
	s.audit(t)
}
