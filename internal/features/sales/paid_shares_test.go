package sales

import (
	"testing"

	"github.com/shopspring/decimal"
)

func items(nets ...string) []SaleItem {
	out := make([]SaleItem, len(nets))
	for i, n := range nets {
		out[i] = SaleItem{ID: int64(i + 1), Subtotal: decimal.RequireFromString(n), Quantity: decimal.NewFromInt(1)}
	}
	return out
}

// DISC-5 / RET-5: the lines' paid shares add up to what the goods were paid,
// to the cent; each share is within a cent of its exact share.
func TestPaidSharesAddUpExactly(t *testing.T) {
	cases := []struct {
		name  string
		total string
		nets  []string
		want  []string
	}{
		{"3 × 100, Rs 10 off", "290", []string{"100", "100", "100"}, []string{"96.67", "96.67", "96.66"}},
		{"no bill discount", "300", []string{"100", "150", "50"}, []string{"100.00", "150.00", "50.00"}},
		{"uneven", "360", []string{"270", "100"}, []string{"262.70", "97.30"}},
		{"payout left out", "-410", []string{"90", "-500"}, []string{"90.00", "-500.00"}},
		{"payout + discount", "-415", []string{"50", "40", "-500"}, []string{"47.22", "37.78", "-500.00"}},
		{"all free", "0", []string{"0", "0"}, []string{"0.00", "0.00"}},
		{"tax lifts it", "315", []string{"100", "100", "100"}, []string{"105.00", "105.00", "105.00"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			its := items(c.nets...)
			got := PaidShares(decimal.RequireFromString(c.total), its)
			for i, it := range its {
				if got[it.ID].StringFixed(2) != c.want[i] {
					t.Errorf("line %d: %s, want %s", i+1, got[it.ID].StringFixed(2), c.want[i])
				}
			}
		})
	}
}

// Over many random-looking sales the shares always add up exactly.
func TestPaidSharesSumToGoodsPaid(t *testing.T) {
	for n := 1; n <= 9; n++ {
		for off := 0; off < 400; off += 7 {
			nets := make([]string, n)
			sum := decimal.Zero
			for i := range nets {
				v := decimal.New(int64(1+(i*3331+n*17)%9973), -2) // 0.01 … 99.73
				nets[i], sum = v.String(), sum.Add(v)
			}
			total := decimal.Max(sum.Sub(decimal.New(int64(off), -2)), decimal.Zero)
			got := PaidShares(total, items(nets...))
			s := decimal.Zero
			for _, v := range got {
				s = s.Add(v)
			}
			if !s.Equal(total) {
				t.Fatalf("%d lines %v, total %s: shares add up to %s", n, nets, total, s)
			}
		}
	}
}

// DISC-1, DISC-2, DISC-3 (a gap mutation testing found): a fixed item discount
// is per unit × qty, a percent one is off the line gross; both clamp to the
// line; a percent bill discount is off its base, rounded once.
func TestDiscountsResolve(t *testing.T) {
	d := decimal.RequireFromString
	cases := []struct {
		name, typ, value, gross, qty, want string
	}{
		{"fixed per unit", "fixed", "5", "250", "3", "15"},
		{"percent of the line", "percent", "5", "250", "3", "12.5"},
		{"percent rounds once", "percent", "12.5", "33.33", "1", "4.17"},
		{"fixed never past the line", "fixed", "150", "300", "3", "300"},
		{"never below zero", "fixed", "-5", "300", "3", "0"},
	}
	for _, c := range cases {
		if got := resolveItemDiscount(c.typ, d(c.value), d(c.gross), d(c.qty)); !got.Equal(d(c.want)) {
			t.Errorf("item %s: %s, want %s", c.name, got, c.want)
		}
	}
	if got := resolveBillDiscount("percent", d("10"), d("333.35")); !got.Equal(d("33.34")) {
		t.Errorf("bill 10%% of 333.35: %s, want 33.34", got)
	}
	if got := resolveBillDiscount("fixed", d("500"), d("120")); !got.Equal(d("120")) {
		t.Errorf("bill never past its base: %s, want 120", got)
	}
}
