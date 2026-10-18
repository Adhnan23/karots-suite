package recharge

import (
	"testing"

	"github.com/shopspring/decimal"
)

var d = decimal.RequireFromString

func TestCommission(t *testing.T) {
	for _, c := range []struct {
		r    Rate
		amt  string
		want string
	}{
		{Rate{Percent: d("3")}, "100", "3"},
		{Rate{Percent: d("2.5"), Fixed: d("5")}, "1000", "30"},
		{Rate{Fixed: d("10")}, "5000", "10"},
		{Rate{Percent: d("3.333")}, "100", "3.33"},
		{Rate{Percent: d("3")}, "0", "0"},
		{Rate{}, "500", "0"},
	} {
		if got := c.r.Commission(d(c.amt)); !got.Equal(d(c.want)) {
			t.Errorf("%+v on %s = %s, want %s", c.r, c.amt, got, c.want)
		}
	}
	// The user's case: a 100 package that uses 88 of float earns 12.
	if got := packageCommission(d("100"), d("88")); !got.Equal(d("12")) {
		t.Errorf("package commission %s, want 12", got)
	}
}

func TestSuggestRate(t *testing.T) {
	ds := func(xs ...string) []decimal.Decimal {
		out := make([]decimal.Decimal, len(xs))
		for i, x := range xs {
			out[i] = d(x)
		}
		return out
	}
	// Counts kept showing 40 extra on 10,000 of reloads at 3%: 3.4%.
	if got, ok := suggestRate(d("3"), ds("15", "25"), d("10000")); !ok || !got.Equal(d("3.4")) {
		t.Fatalf("suggest = %s %v, want 3.4", got, ok)
	}
	for name, c := range map[string]struct {
		diffs   []decimal.Decimal
		reloads string
	}{
		"one count":         {ds("40"), "10000"},
		"mixed signs":       {ds("40", "-10"), "10000"},
		"too small a share": {ds("5", "5"), "10000"},
		"no reloads":        {ds("40", "40"), "0"},
		"a zero count":      {ds("40", "0"), "10000"},
	} {
		if _, ok := suggestRate(d("3"), c.diffs, d(c.reloads)); ok {
			t.Errorf("%s: suggested a rate", name)
		}
	}
	if got, ok := suggestRate(d("0.5"), ds("-60", "-60"), d("10000")); !ok || !got.Equal(d("0")) {
		t.Errorf("never below zero: %s %v", got, ok)
	}
}
