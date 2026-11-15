package combos

import (
	"testing"

	"github.com/shopspring/decimal"
)

// A frozen part sells from the uncounted shelf: no "only N in stock" warning.
func TestWarningsSkipFrozen(t *testing.T) {
	c := Combo{Items: []Item{
		{Name: "Pencil", Qty: decimal.NewFromInt(2), Active: true, Stock: decimal.Zero, Frozen: true},
		{Name: "Eraser", Qty: decimal.NewFromInt(1), Active: true, Stock: decimal.Zero},
	}}
	if w := c.Warnings(); len(w) != 1 || w[0] != "Eraser: 0 in stock" {
		t.Fatalf("%v", w)
	}
}
