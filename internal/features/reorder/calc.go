// Package reorder learns each product's reorder level from how it sells and how
// often it is delivered (docs/specs/2026-10-21-smart-reorder-levels-design.md).
// calc.go is the arithmetic, on plain inputs; run.go loads them and writes the
// results nightly. Core only: low stock and reorder_level are core.
package reorder

import (
	"math"
	"slices"
	"time"
)

// The numbers the spec settles. One place, so they can be tuned together.
const (
	recentDays     = 30  // sales speed window…
	fallbackDays   = 90  // …or this, when the product was in stock under minInStockDays of it
	minInStockDays = 10  //
	deliveryDays   = 180 // deliveries looked at for the supplier's rhythm
	minDeliveries  = 3   // fewer → the next source (product → supplier → shop default)
	gapsLooked     = 4   // an irregular supplier plans for the longest of the last 4 gaps
	irregular      = 1.5 // longest gap over 1.5 × the median = irregular
	bigBill        = 3.0 // a bill over 3 × the usual (75th percentile) counts as the median
	minBills       = 5   // fewer bills → nothing is capped
	learnDays      = 56  // an Auto level is fully learned after 8 weeks of history
	offFactor      = 2   // a Fixed level is "off" at ≥ 2× or ≤ ½ what sales suggest…
	offUnits       = 3   // …and at least 3 units apart
)

// Sale is one sale's quantity of the product (base units, returns taken off).
type Sale struct {
	At  time.Time
	Qty float64
}

// Move is a signed stock movement, for the in-stock rebuild (Input.Moves).
type Move struct {
	At  time.Time
	Qty float64
}

// Input is everything known about one product. Sales and Moves go back far
// enough to cover last year's window (the loader takes ~400 days).
type Input struct {
	Sales []Sale
	// Derived is the sales of products made from this one (loose rice from a
	// bag), already in this product's units (÷ the conversion ratio): the
	// original is what gets reordered. Never capped.
	Derived []Sale
	Moves   []Move // every stock movement, to rebuild which days it was on the shelf
	OnHand  float64
	// Delivery times (one per day is enough) of this product from any supplier,
	// and of its preferred supplier's deliveries of anything.
	OwnDeliveries, SupplierDeliveries []time.Time
	DefaultDays, SafetyDays           int
	Since                             time.Time // first sale / delivery, or when it was created
}

// Calc is one product's result, stored as products.reorder_calc (the reason lines).
type Calc struct {
	Daily         float64 `json:"daily"`
	RecentDaily   float64 `json:"recentDaily"`
	LastYearDaily float64 `json:"lastYearDaily"`
	RhythmDays    int     `json:"rhythmDays"`
	RhythmFrom    string  `json:"rhythmFrom"` // product | supplier | default
	Regular       bool    `json:"regular"`
	SafetyDays    int     `json:"safetyDays"`
	CalcLevel     int     `json:"calcLevel"`
	LearnedPct    int     `json:"learnedPct"`
	HistoryDays   int     `json:"historyDays"`
	NotSelling    bool    `json:"notSelling"`
	CappedBills   int     `json:"cappedBills"` // one-off big bills left out of the last 90 days' speed
}

// day is how many whole days before now t falls (0 = the last 24 hours).
func day(now, t time.Time) int { return int(now.Sub(t) / (24 * time.Hour)) }

// Compute works out one product's level from its history.
func Compute(in Input, now time.Time) Calc {
	c := Calc{SafetyDays: in.SafetyDays, HistoryDays: max(0, day(now, in.Since))}
	c.LearnedPct = min(100, c.HistoryDays*100/learnDays)

	// Use per day ago: sales (one-off big bills capped) plus derived products' sales.
	use := map[int]float64{}
	usual, median := billSizes(in.Sales)
	for _, s := range in.Sales {
		q, d := s.Qty, day(now, s.At)
		if usual > 0 && q > bigBill*usual {
			q = median
			if d < fallbackDays {
				c.CappedBills++
			}
		}
		use[d] += q
	}
	for _, s := range in.Derived {
		use[day(now, s.At)] += s.Qty
	}

	// Stock at the start of each day d days ago, rebuilt backwards from today's
	// (one pass: start of day d = on hand − every move from that day on).
	ly := day(now, now.AddDate(-1, 0, 0))
	byDay := map[int]float64{}
	for _, m := range in.Moves {
		byDay[day(now, m.At)] += m.Qty
	}
	start := make([]float64, max(fallbackDays, ly+1)+1)
	for d, s := 0, in.OnHand; d < len(start); d++ {
		s -= byDay[d]
		start[d] = s
	}
	inStock := func(d int) bool {
		if d < 0 || d >= len(start) || (d >= c.HistoryDays && d > 0) {
			return false // before it existed (or outside what was loaded)
		}
		return use[d] > 0 || start[d] > 0
	}
	speed := func(from, to int) (float64, int) { // days [from, to) ago
		var u float64
		n := 0
		for d := from; d < to; d++ {
			if inStock(d) {
				u += use[d]
				n++
			}
		}
		if n == 0 {
			return 0, 0
		}
		return u / float64(n), n
	}

	c.RhythmDays, c.RhythmFrom, c.Regular = rhythm(in, now)
	var n int
	if c.RecentDaily, n = speed(0, recentDays); n < minInStockDays {
		c.RecentDaily, _ = speed(0, fallbackDays)
	}
	span := c.RhythmDays + c.SafetyDays
	c.LastYearDaily, _ = speed(ly-span+1, ly+1)
	c.Daily = math.Max(c.RecentDaily, c.LastYearDaily)

	sold90 := false
	for d := range fallbackDays {
		sold90 = sold90 || use[d] > 0
	}
	c.NotSelling = !sold90 && c.LastYearDaily == 0
	if !c.NotSelling {
		c.CalcLevel = int(math.Ceil(round2(c.Daily * float64(span))))
	}
	return c
}

// billSizes is the usual (75th percentile) and median bill size, or 0, 0 with
// too few bills to tell.
func billSizes(sales []Sale) (usual, median float64) {
	if len(sales) < minBills {
		return 0, 0
	}
	q := make([]float64, len(sales))
	for i, s := range sales {
		q[i] = s.Qty
	}
	slices.Sort(q)
	return q[len(q)*3/4], q[len(q)/2]
}

// rhythm is the days between deliveries: the product's own, else its supplier's,
// else the shop default. Regular → the median gap; irregular → the longest of
// the last few gaps.
func rhythm(in Input, now time.Time) (days int, from string, regular bool) {
	for _, src := range []struct {
		name  string
		times []time.Time
	}{{"product", in.OwnDeliveries}, {"supplier", in.SupplierDeliveries}} {
		var ds []int
		for _, t := range src.times {
			if d := day(now, t); d < deliveryDays && !slices.Contains(ds, d) {
				ds = append(ds, d)
			}
		}
		if len(ds) < minDeliveries {
			continue
		}
		slices.Sort(ds) // most recent first
		gaps := make([]int, len(ds)-1)
		for i := range gaps {
			gaps[i] = ds[i+1] - ds[i]
		}
		last := slices.Max(gaps[:min(gapsLooked, len(gaps))])
		sorted := slices.Clone(gaps)
		slices.Sort(sorted)
		med := sorted[len(sorted)/2]
		if float64(last) > irregular*float64(med) {
			return last, src.name, false
		}
		return med, src.name, true
	}
	return in.DefaultDays, "default", true
}

// Blend moves an Auto level from the starting guess to the learned one over
// learnDays of history. No guess (0) → the learned level from day one.
func Blend(start, calcLevel, historyDays int) int {
	if start == 0 {
		return calcLevel
	}
	w := math.Min(1, float64(historyDays)/learnDays)
	return int(math.Round(float64(start)*(1-w) + float64(calcLevel)*w))
}

// LooksOff reports a Fixed level far from what fully learned sales suggest.
func LooksOff(level int, c Calc) bool {
	if c.LearnedPct < 100 || abs(c.CalcLevel-level) < offUnits {
		return false
	}
	return c.CalcLevel >= offFactor*level || offFactor*c.CalcLevel <= level
}

func abs(n int) int { return max(n, -n) }

// round2 keeps float noise (9.999…) from ceiling a whole number up.
func round2(f float64) float64 { return math.Round(f*100) / 100 }
