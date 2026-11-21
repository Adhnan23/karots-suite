package clearance

import (
	"testing"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

func dec(s string) decimal.Decimal { d, _ := decimal.NewFromString(s); return d }

func TestSuggestPercentFlooredAtCost(t *testing.T) {
	cases := []struct {
		name                               string
		sell, cost, deflt, minMargin, want string
	}{
		// 20% off 120 = 96, but floor = cost*1.05 = 105 → clamp to 12.5%.
		{"clamped to floor", "120", "100", "20", "5", "12.5"},
		// 20% off 200 = 160 >= floor 105 → full 20%.
		{"unclamped", "200", "100", "20", "5", "20"},
		// price already at/under floor → nothing safe to give.
		{"already cheap", "100", "100", "20", "5", "0"},
		// floor 945 of 1150 allows 17.826…% — cut down to one decimal, never rounded up past the floor.
		{"one decimal, rounded down", "1150", "900", "20", "5", "17.8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := suggestPercent(dec(c.sell), dec(c.cost), dec(c.deflt), dec(c.minMargin))
			if !got.Equal(dec(c.want)) {
				t.Errorf("suggestPercent = %s, want %s", got, c.want)
			}
		})
	}
}

func TestNewPrice(t *testing.T) {
	if got := newPrice(dec("120"), dec("12.5")); !got.Equal(dec("105")) {
		t.Errorf("newPrice = %s, want 105", got)
	}
}

// CLR-3: for any price, cost, default and margin, the suggested price never
// falls under the floor, and the suggestion is the default whenever that fits.
func TestCLR3_SuggestionNeverUnderFloor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cents := func(name string, lo, hi int64) decimal.Decimal {
			return decimal.New(rapid.Int64Range(lo, hi).Draw(t, name), -2)
		}
		sell, cost := cents("sell", 1, 10_000_000), cents("cost", 0, 10_000_000)
		def, margin := decimal.New(rapid.Int64Range(1, 999).Draw(t, "def"), -1), decimal.New(rapid.Int64Range(0, 500).Draw(t, "margin"), -1)
		pct := suggestPercent(sell, cost, def, margin)
		if pct.IsNegative() || pct.GreaterThan(def) {
			t.Fatalf("suggested %s%%, default %s%%", pct, def)
		}
		floor := cost.Mul(decimal.NewFromInt(100).Add(margin)).Div(decimal.NewFromInt(100))
		after := sell.Mul(decimal.NewFromInt(100).Sub(pct)).Div(decimal.NewFromInt(100))
		if pct.IsPositive() && after.LessThan(floor) {
			t.Fatalf("%s at %s%% off is %s, under the floor %s", sell, pct, after, floor)
		}
		if atDef := sell.Mul(decimal.NewFromInt(100).Sub(def)).Div(decimal.NewFromInt(100)); !atDef.LessThan(floor) && !pct.Equal(def) {
			t.Fatalf("default %s%% fits (%s ≥ %s) but %s%% suggested", def, atDef, floor, pct)
		}
	})
}
