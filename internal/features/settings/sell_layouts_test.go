package settings

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestValidSellLayouts(t *testing.T) {
	t.Run("a good layout is kept and its shares add up to one", func(t *testing.T) {
		got, err := ValidSellLayouts(map[string]SellLayout{"1366x768": {Arr: "PCY", Fr: []float64{42, 29, 29}, Card: "M"}})
		if err != nil {
			t.Fatal(err)
		}
		l := got["1366x768"]
		if l.Arr != "PCY" || l.Card != "M" || l.Text != 100 || len(l.Fr) != 3 {
			t.Fatalf("got %+v", l)
		}
		if s := l.Fr[0] + l.Fr[1] + l.Fr[2]; math.Abs(s-1) > 1e-9 {
			t.Fatalf("shares sum to %v", s)
		}
	})
	bad := []struct {
		name string
		in   map[string]SellLayout
		msg  string
	}{
		{"unknown arrangement", map[string]SellLayout{"1366x768": {Arr: "PXY", Fr: []float64{.4, .3, .3}}}, "arrangement"},
		{"key that isn't a size", map[string]SellLayout{"big": {Arr: "PCY", Fr: []float64{.4, .3, .3}}}, "size"},
		{"size too small", map[string]SellLayout{"99x99": {Arr: "PCY", Fr: []float64{.4, .3, .3}}}, "size"},
		{"wrong share count", map[string]SellLayout{"1366x768": {Arr: "P_CY", Fr: []float64{.4, .3, .3}, Split: .5}}, "share"},
		{"payment too narrow", map[string]SellLayout{"1366x768": {Arr: "PCY", Fr: []float64{.45, .3, .25}}}, "Payment"},
		{"split out of range", map[string]SellLayout{"1366x768": {Arr: "P_CY", Fr: []float64{.5, .5}, Split: .9}}, "split"},
		{"unknown card size", map[string]SellLayout{"1366x768": {Arr: "PCY", Fr: []float64{.4, .3, .3}, Card: "X"}}, "card"},
		{"unknown text size", map[string]SellLayout{"1366x768": {Arr: "PCY", Fr: []float64{.4, .3, .3}, Text: 95}}, "text"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			_, err := ValidSellLayouts(c.in)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.msg)) {
				t.Fatalf("want an error about %q, got %v", c.msg, err)
			}
		})
	}
	t.Run("defaults: card L, text 100, split dropped on three columns", func(t *testing.T) {
		got, err := ValidSellLayouts(map[string]SellLayout{"1920x1080": {Arr: "CPY", Fr: []float64{.3, .45, .25}, Split: .7}})
		if err != nil {
			t.Fatal(err)
		}
		if l := got["1920x1080"]; l.Card != "L" || l.Text != 100 || l.Split != 0 {
			t.Fatalf("got %+v", l)
		}
	})
	t.Run("a stacked column needs the wider of its two", func(t *testing.T) {
		// 1280 wide: inner 1280-28-14 = 1238; .25 → 309 px < Cart's 340.
		_, err := ValidSellLayouts(map[string]SellLayout{"1280x1024": {Arr: "P_CY", Fr: []float64{.75, .25}, Split: .5}})
		if err == nil || !strings.Contains(err.Error(), "Cart") {
			t.Fatalf("want Cart too narrow, got %v", err)
		}
	})
	t.Run("empty is fine (every size reset)", func(t *testing.T) {
		got, err := ValidSellLayouts(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v %v", got, err)
		}
	})
}

// A bar the Appearance card dragged hard against two minimums must still save (float
// rounding put 339.99999999999994 below 340).
func TestSellLayoutsAtTheirMinimums(t *testing.T) {
	for w := 1000; w <= 2560; w++ {
		inner := float64(w - 2*sellPad - 2*sellGap)
		fr := []float64{1 - (340+330)/inner, 340 / inner, 330 / inner}
		if fr[0]*inner < 300 {
			continue
		}
		if _, err := ValidSellLayouts(map[string]SellLayout{fmt.Sprintf("%dx768", w): {Arr: "PCY", Fr: fr}}); err != nil {
			t.Fatalf("%d wide: %v", w, err)
		}
	}
}

// Saved layouts a later build wrote in another shape read as none (today's layout),
// never as an error that takes down every settings read.
func TestSellLayoutsScanJunk(t *testing.T) {
	var l SellLayouts
	if err := l.Scan([]byte(`{"1366x768":{"arr":"PCY","fr":{"a":1},"text":105.5}}`)); err != nil {
		t.Fatalf("junk returned an error: %v", err)
	}
	if len(l) != 0 {
		t.Fatalf("junk kept: %v", l)
	}
}

func TestSellArrangementsHoldEachSectionOnce(t *testing.T) {
	if len(SellArrangements) != 18 {
		t.Fatalf("%d arrangements, want 18 (the SPA's list)", len(SellArrangements))
	}
	for id, cols := range SellArrangements {
		seen := map[rune]int{}
		for _, c := range cols {
			for _, r := range c {
				seen[r]++
			}
		}
		if seen['P'] != 1 || seen['C'] != 1 || seen['Y'] != 1 || len(seen) != 3 {
			t.Errorf("%s: %v", id, cols)
		}
	}
}
