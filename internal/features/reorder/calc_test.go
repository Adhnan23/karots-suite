package reorder

import (
	"testing"
	"time"
)

var now = time.Date(2026, 10, 21, 3, 0, 0, 0, time.Local)

func ago(days int) time.Time { return now.AddDate(0, 0, -days).Add(-time.Hour) }

// daily sales of qty for days [from, to) ago, one bill a day, stock always on hand.
func steady(from, to int, qty float64) []Sale {
	var s []Sale
	for d := to - 1; d >= from; d-- {
		s = append(s, Sale{At: ago(d), Qty: qty})
	}
	return s
}

func base() Input {
	return Input{OnHand: 1000, DefaultDays: 7, SafetyDays: 3, Since: ago(365)}
}

func deliveriesEvery(days, n int) []time.Time {
	var out []time.Time
	for i := n; i >= 1; i-- {
		out = append(out, ago(i*days))
	}
	return out
}

func TestSteadySellerWithWeeklySupplier(t *testing.T) {
	in := base()
	in.Sales = steady(0, 90, 10)
	in.OwnDeliveries = deliveriesEvery(7, 6)
	c := Compute(in, now)
	if c.Daily != 10 || c.RhythmDays != 7 || c.RhythmFrom != "product" || !c.Regular || c.CalcLevel != 100 {
		t.Fatalf("got daily %v rhythm %d from %s regular %v level %d, want 10 7 product true 100", c.Daily, c.RhythmDays, c.RhythmFrom, c.Regular, c.CalcLevel)
	}
}

func TestOneOffBigBillIsCapped(t *testing.T) {
	in := base()
	in.Sales = append(steady(0, 30, 2), Sale{At: ago(3), Qty: 200})
	c := Compute(in, now)
	if c.CappedBills != 1 || c.RecentDaily > 2.1 {
		t.Fatalf("capped %d recent %v, want 1 bill capped and ~2 a day", c.CappedBills, c.RecentDaily)
	}
}

func TestRegularBigBuyerIsNotCapped(t *testing.T) {
	in := base()
	for d := 29; d >= 0; d-- {
		q := 2.0
		if d%2 == 0 {
			q = 20 // a school buys 20 every other day
		}
		in.Sales = append(in.Sales, Sale{At: ago(d), Qty: q})
	}
	c := Compute(in, now)
	if c.CappedBills != 0 || c.RecentDaily < 10 {
		t.Fatalf("capped %d recent %v, want none capped, ~11 a day", c.CappedBills, c.RecentDaily)
	}
}

func TestOutOfStockDaysDoNotSlowTheSpeed(t *testing.T) {
	in := base()
	in.OnHand = 0
	in.Sales = steady(15, 30, 10) // sold out 15 days ago, nothing since
	// Stock: 150 arrived 30 days ago, sold 10 a day down to 0.
	in.Moves = []Move{{At: ago(31), Qty: 150}}
	for _, s := range in.Sales {
		in.Moves = append(in.Moves, Move{At: s.At, Qty: -s.Qty})
	}
	c := Compute(in, now)
	if c.RecentDaily < 9.9 {
		t.Fatalf("recent %v, want 10 (15 in-stock days only)", c.RecentDaily)
	}
}

func TestFewInStockDaysFallBackTo90(t *testing.T) {
	in := base()
	in.OnHand = 0
	// 60 in-stock days long ago at 4 a day, then out for the last 30.
	in.Sales = steady(31, 91, 4)
	in.Moves = []Move{{At: ago(92), Qty: 240}}
	for _, s := range in.Sales {
		in.Moves = append(in.Moves, Move{At: s.At, Qty: -s.Qty})
	}
	if c := Compute(in, now); c.RecentDaily < 3.9 || c.RecentDaily > 4.1 {
		t.Fatalf("recent %v, want 4 from the 90-day window", c.RecentDaily)
	}
}

func TestIrregularSupplierPlansForTheLongestGap(t *testing.T) {
	in := base()
	in.Sales = steady(0, 90, 1)
	in.OwnDeliveries = []time.Time{ago(33), ago(28), ago(23), ago(18), ago(4)} // gaps 5 5 5 14
	c := Compute(in, now)
	if c.Regular || c.RhythmDays != 14 {
		t.Fatalf("regular %v rhythm %d, want irregular 14", c.Regular, c.RhythmDays)
	}
}

func TestRhythmFallsBackToSupplierThenDefault(t *testing.T) {
	in := base()
	in.Sales = steady(0, 90, 1)
	in.OwnDeliveries = []time.Time{ago(10)}
	in.SupplierDeliveries = deliveriesEvery(5, 8)
	if c := Compute(in, now); c.RhythmFrom != "supplier" || c.RhythmDays != 5 {
		t.Fatalf("from %s %d, want supplier 5", c.RhythmFrom, c.RhythmDays)
	}
	in.SupplierDeliveries = nil
	if c := Compute(in, now); c.RhythmFrom != "default" || c.RhythmDays != 7 {
		t.Fatalf("from %s %d, want default 7", c.RhythmFrom, c.RhythmDays)
	}
}

func TestLastYearWindowWins(t *testing.T) {
	in := base()
	in.Since = ago(500)
	in.Sales = append(steady(0, 30, 2), steady(355, 366, 15)...) // the coming 10 days, last year
	c := Compute(in, now)
	if c.LastYearDaily < 14.9 || c.Daily != c.LastYearDaily || c.CalcLevel != 150 {
		t.Fatalf("last year %v daily %v level %d, want 15 15 150", c.LastYearDaily, c.Daily, c.CalcLevel)
	}
}

func TestDerivedSalesCountForTheOriginal(t *testing.T) {
	in := base()
	in.Sales = steady(0, 30, 1)     // 1 bag a day sold whole
	in.Derived = steady(0, 30, 0.5) // + 12.5 kg loose a day = ½ bag
	if c := Compute(in, now); c.RecentDaily != 1.5 || c.NotSelling {
		t.Fatalf("recent %v notSelling %v, want 1.5 bags a day", c.RecentDaily, c.NotSelling)
	}
}

func TestNotSelling(t *testing.T) {
	in := base()
	in.Sales = steady(200, 210, 3)
	c := Compute(in, now)
	if !c.NotSelling || c.CalcLevel != 0 {
		t.Fatalf("notSelling %v level %d, want true 0", c.NotSelling, c.CalcLevel)
	}
}

func TestBrandNewProduct(t *testing.T) {
	in := base()
	in.OnHand = 0
	in.Since = now
	c := Compute(in, now)
	if c.HistoryDays != 0 || c.LearnedPct != 0 || c.CalcLevel != 0 {
		t.Fatalf("history %d learned %d level %d, want 0 0 0", c.HistoryDays, c.LearnedPct, c.CalcLevel)
	}
	if got := Blend(5, c.CalcLevel, c.HistoryDays); got != 5 {
		t.Fatalf("Blend = %d, want the starting guess 5", got)
	}
}

func TestBlend(t *testing.T) {
	cases := []struct{ start, calc, days, want int }{
		{20, 100, 0, 20}, {20, 100, 28, 60}, {20, 100, 56, 100}, {20, 100, 400, 100}, {0, 37, 0, 37},
	}
	for _, c := range cases {
		if got := Blend(c.start, c.calc, c.days); got != c.want {
			t.Errorf("Blend(%d,%d,%d) = %d, want %d", c.start, c.calc, c.days, got, c.want)
		}
	}
}

func TestLooksOff(t *testing.T) {
	learned := func(level int) Calc { return Calc{CalcLevel: level, LearnedPct: 100} }
	cases := []struct {
		level int
		c     Calc
		want  bool
	}{
		{20, learned(6), true},
		{10, learned(60), true},
		{10, learned(15), false},                        // not 2×
		{2, learned(4), false},                          // 2× but under 3 units apart
		{20, Calc{CalcLevel: 6, LearnedPct: 50}, false}, // still learning
		{0, learned(5), true},
	}
	for _, c := range cases {
		if got := LooksOff(c.level, c.c); got != c.want {
			t.Errorf("LooksOff(%d, %+v) = %v, want %v", c.level, c.c, got, c.want)
		}
	}
}

// A whole season of big bills (last year's school rush) is that season's normal,
// not one-off bills — even when the rest of the year sells far less.
func TestSeasonalRushIsNotCapped(t *testing.T) {
	in := base()
	in.Since = ago(420)
	in.Sales = append(append(steady(0, 345, 2), steady(345, 368, 15)...), steady(368, 400, 2)...)
	in.OwnDeliveries = deliveriesEvery(10, 10)
	c := Compute(in, now)
	if c.LastYearDaily < 14.9 || c.CalcLevel != 195 {
		t.Fatalf("last year %v level %d, want 15 a day → 195", c.LastYearDaily, c.CalcLevel)
	}
}
