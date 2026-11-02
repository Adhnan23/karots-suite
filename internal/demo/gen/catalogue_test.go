package gen

import (
	"strings"
	"testing"
)

// A mixed shop buys each category from a supplier of its own kind.
func TestCategorySuppliersKeepToTheirList(t *testing.T) {
	_, sups := linesFor("mixed")
	bySup := categorySuppliers("mixed")
	for cat, want := range map[string]string{
		"Antibiotics": "State Pharmaceuticals|Hemas|Emerchemie", "Heart & BP": "State Pharmaceuticals|Hemas|Emerchemie",
		"Snacks": "Colombo|Kandy|Lanka Dairies|Ceylon Beverages", "Phones": "Mobile Hub|Pettah|SIM",
		"Cement & Sand": "Tokyo|Lanka Steel|S-Lon|Electro Mart",
	} {
		i, ok := bySup[cat]
		if !ok {
			t.Fatalf("%s has no supplier", cat)
		}
		match := false
		for _, name := range strings.Split(want, "|") {
			match = match || strings.HasPrefix(sups[i], name)
		}
		if !match {
			t.Errorf("%s comes from %s", cat, sups[i])
		}
	}
	if got := supplierOf(bySup, "Print & Copy", 5, 3); got != 2 {
		t.Errorf("a category on no list: %d", got)
	}
}
