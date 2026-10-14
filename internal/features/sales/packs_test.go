package sales

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/shopspring/decimal"
)

// The same table drives web cart/packs.test.ts, so the till preview and the
// booked line can't disagree.
func TestPackGross(t *testing.T) {
	raw, err := os.ReadFile("../../../web/src/features/till/cart/packs.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Case, Qty, Unit, Gross, Label string
		Wholesale                     bool
		Packs                         []struct{ Name, Size, Selling, Wholesale string }
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	d := decimal.RequireFromString
	for _, c := range cases {
		var ks []PackPrice
		for _, k := range c.Packs {
			ks = append(ks, PackPrice{Name: k.Name, Size: d(k.Size), Selling: d(k.Selling), Wholesale: d(k.Wholesale)})
		}
		gross, label := PackGross(d(c.Qty), d(c.Unit), ks, c.Wholesale)
		if gross.StringFixed(2) != c.Gross || label != c.Label {
			t.Errorf("%s: got %s %q, want %s %q", c.Case, gross.StringFixed(2), label, c.Gross, c.Label)
		}
	}
}
