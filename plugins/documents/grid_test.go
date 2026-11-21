package documents

import (
	"strings"
	"testing"
)

// DOC-3: names at their limits are taken, one letter more is refused; an extra
// with no stock product (0) is a plain charge.
func TestGridAndExtrasLimits(t *testing.T) {
	row := func(size, material string) GridDTO {
		return GridDTO{Rows: []GridRow{{Size: size, Material: material, Tiers: []GridTier{{MinQty: 1, Cells: [4]string{"5"}}}}}}
	}
	for _, c := range []struct {
		size, material string
		ok             bool
	}{{strings.Repeat("s", 20), "", true}, {strings.Repeat("s", 21), "", false}, {"A4", strings.Repeat("m", 30), true}, {"A4", strings.Repeat("m", 31), false}} {
		if _, _, err := fromGrid(row(c.size, c.material)); (err == nil) != c.ok {
			t.Errorf("size %d letters, type %d letters: err %v, want ok %v", len(c.size), len(c.material), err, c.ok)
		}
	}
	for _, c := range []struct {
		name string
		ok   bool
	}{{strings.Repeat("n", 40), true}, {strings.Repeat("n", 41), false}} {
		if _, err := fromExtras([]ExtraDTO{{Name: c.name, Price: "1"}}); (err == nil) != c.ok {
			t.Errorf("extra name %d letters: err %v, want ok %v", len(c.name), err, c.ok)
		}
	}
	zero := int64(0)
	xs, err := fromExtras([]ExtraDTO{{Name: "Urgent", Price: "1", ProductID: &zero}})
	if err != nil || xs[0].ProductID != nil {
		t.Errorf("an extra with product 0 is a plain charge: %+v, %v", xs, err)
	}
	if _, _, err := fromGrid(GridDTO{Rows: []GridRow{{Size: ""}, {Size: " "}}}); err == nil || !strings.Contains(err.Error(), "Any size") {
		t.Errorf("any size twice: %v, want it named Any size", err)
	}
}

// DOC-2: the grid reads back with the any-size row first and each size by name.
func TestToGridOrder(t *testing.T) {
	a4 := "A4"
	g := toGrid([]Price{{Size: &a4, MinQty: 1}, {Size: nil, MinQty: 1}}, nil)
	if len(g.Rows) != 2 || g.Rows[0].Size != "" || g.Rows[1].Size != "A4" {
		t.Errorf("rows %+v, want any size first, then A4", g.Rows)
	}
}
