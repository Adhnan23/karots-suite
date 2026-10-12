package documents

import (
	"testing"

	"karots-suite/internal/features/recipes"

	"github.com/shopspring/decimal"
)

func sp(s string) *string         { return &s }
func dd(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestPickPrice(t *testing.T) {
	rows := []Price{
		{ID: 1, Size: nil, MinQty: 1, UnitPrice: dd("10")},       // any size, B&W 1-side
		{ID: 2, Size: sp("A4"), MinQty: 1, UnitPrice: dd("8")},   // A4 base
		{ID: 3, Size: sp("A4"), MinQty: 100, UnitPrice: dd("6")}, // A4 from 100
		{ID: 4, Size: nil, MinQty: 500, UnitPrice: dd("4")},      // any size from 500
		{ID: 5, Size: sp("A4"), Color: true, MinQty: 1, UnitPrice: dd("40")},
		{ID: 6, Size: sp("A3"), DoubleSide: true, MinQty: 1, UnitPrice: dd("30")},
	}
	for _, c := range []struct {
		o    JobOptions
		want int64
	}{
		{JobOptions{Size: "A4", Copies: 20}, 2},
		{JobOptions{Size: "A4", Copies: 100}, 3},
		{JobOptions{Size: "A4", Copies: 600}, 3}, // the size's own row beats a bigger any-size tier
		{JobOptions{Size: "Legal", Copies: 5}, 1},
		{JobOptions{Size: "Legal", Copies: 500}, 4},
		{JobOptions{Size: "A4", Color: true, Copies: 3}, 5},
		{JobOptions{Size: "A3", Double: true, Copies: 3}, 6},
		{JobOptions{Size: "A3", Color: true, Copies: 3}, 0}, // not offered
	} {
		got, ok := PickPrice(rows, c.o)
		if c.want == 0 {
			if ok {
				t.Errorf("%+v: got row %d, want none", c.o, got.ID)
			}
			continue
		}
		if !ok || got.ID != c.want {
			t.Errorf("%+v: got row %d (%v), want %d", c.o, got.ID, ok, c.want)
		}
	}
}

func TestComponents(t *testing.T) {
	paper := []Paper{
		{Size: nil, ProductID: 10, QtyPerUnit: dd("1"), CostPrice: dd("1")},
		{Size: sp("A3"), ProductID: 11, QtyPerUnit: dd("1"), CostPrice: dd("3")},
	}
	toner := recipes.Component{ComponentProductID: 20, YieldUnits: decimal.NullDecimal{Decimal: dd("5000"), Valid: true}, CostPrice: dd("10000")}
	again := recipes.Component{ComponentProductID: 10, QtyPerUnit: decimal.NullDecimal{Decimal: dd("1"), Valid: true}, WholeUnits: true, CostPrice: dd("1")}

	// 21 copies double-sided on A4 (any-size paper): 11 sheets; toner per copy, fractional;
	// the paper the recipe names again is not taken twice.
	cs, cost := Components(PaperFor(paper, "A4"), []recipes.Component{toner, again}, JobOptions{Size: "A4", Double: true, Copies: 21})
	if len(cs) != 2 || cs[0].ProductID != 10 || !cs[0].Qty.Equal(dd("11")) || cs[1].ProductID != 20 || !cs[1].Qty.Equal(dd("0.0042")) {
		t.Fatalf("components %+v", cs)
	}
	if !cost.Equal(dd("53")) { // 11 sheets + 0.0042 × 10000
		t.Errorf("cost %s, want 53", cost)
	}
	// A3 has its own paper, so the any-size paper isn't used.
	cs, _ = Components(PaperFor(paper, "A3"), nil, JobOptions{Size: "A3", Copies: 1})
	if len(cs) != 1 || cs[0].ProductID != 11 || !cs[0].Qty.Equal(dd("1")) {
		t.Fatalf("A3 components %+v", cs)
	}
}
