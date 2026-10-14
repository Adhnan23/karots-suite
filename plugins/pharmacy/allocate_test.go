package pharmacy

import "testing"

// Tablets per script line come from the sale: tagged lines first, then lines
// sharing the line's medicine group (any brand), card + loose added up; days =
// tablets ÷ doses a day, on every row of that line. Other medicines are kept
// off-script; soap is not a medicine.
func TestAllocate(t *testing.T) {
	g1, g2 := int64(1), int64(2)
	pana := int64(30)
	slots := []slot{
		{ID: 100, GroupID: &g1, PerDay: dec(2)},     // Metformin 1-0-1
		{ID: 200, ProductID: &pana, PerDay: dec(3)}, // Panadol, no group
		{ID: 300, GroupID: &g2, PerDay: dec(1)},     // Amlodipine — not bought today
	}
	lines := []saleLine{
		{ProductID: 10, Name: "Glucophage", Qty: dec(12), GroupID: &g1, Medicine: true},
		{ProductID: 11, Name: "Metfor", Qty: dec(10), GroupID: &g1, Medicine: true},
		{ProductID: 30, Name: "Panadol", Qty: dec(9), Medicine: true},
		{ProductID: 40, Name: "Cetirizine", Qty: dec(5), Medicine: true},
		{ProductID: 50, Name: "Soap", Qty: dec(1)},
		{ProductID: 60, Name: "Vitamin C", Qty: dec(30), Medicine: true},
		{ProductID: 70, Name: "Returned", Qty: dec(0), GroupID: &g1, Medicine: true},
	}
	parts := allocate(slots, lines, map[int64]int64{60: 300})
	got := map[int64]part{}
	for _, p := range parts {
		got[p.ProductID] = p
	}
	want := map[int64]part{
		10: {RxItemID: 100, Qty: dec(12), Days: 11}, 11: {RxItemID: 100, Qty: dec(10), Days: 11},
		30: {RxItemID: 200, Qty: dec(9), Days: 3},
		40: {RxItemID: 0, Qty: dec(5), Days: 0},
		60: {RxItemID: 300, Qty: dec(30), Days: 30}, // the cashier tagged it to the script line
	}
	if len(parts) != len(want) {
		t.Fatalf("parts %+v", parts)
	}
	for id, w := range want {
		g := got[id]
		if g.RxItemID != w.RxItemID || !g.Qty.Equal(w.Qty) || g.Days != w.Days {
			t.Errorf("product %d: %+v, want %+v", id, g, w)
		}
	}
}
