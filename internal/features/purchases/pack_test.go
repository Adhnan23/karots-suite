package purchases

import (
	"testing"

	"github.com/shopspring/decimal"
)

// 3 cards @ 250 with 1 free card and Rs 10 off each card: the supplier is owed
// 750 − 30 = 720 exactly; stock comes in as 48 tablets at 20.8333 each.
func TestParseLinesByThePack(t *testing.T) {
	lines, subtotal, err := parseLines([]ItemInput{{ProductID: 1, Quantity: "3", FreeQty: "1", CostPrice: "250",
		Discount: "10", DiscountType: "fixed", PackSize: "12"}})
	if err != nil {
		t.Fatal(err)
	}
	l := lines[0]
	if subtotal.String() != "720" || l.Subtotal.String() != "720" || l.Discount.String() != "30" {
		t.Fatalf("payable %s / line %s / discount %s", subtotal, l.Subtotal, l.Discount)
	}
	if l.Quantity.String() != "36" || l.FreeQty.String() != "12" || l.CostPrice.String() != "20.8333" || l.DiscountValue.String() != "0.83" {
		t.Fatalf("stored %s + %s free @ %s (disc %s)", l.Quantity, l.FreeQty, l.CostPrice, l.DiscountValue)
	}
	if _, _, err := parseLines([]ItemInput{{ProductID: 1, Quantity: "1", CostPrice: "1", PackSize: "1"}}); err == nil {
		t.Fatal("pack size 1 accepted")
	}
}

// A big box's unit cost keeps enough decimals that the lot adds back up to its bill.
func TestBoxUnitCostAddsBackUp(t *testing.T) {
	lines, _, err := parseLines([]ItemInput{{ProductID: 1, Quantity: "1", CostPrice: "1000", PackSize: "120"}})
	if err != nil {
		t.Fatal(err)
	}
	l := lines[0]
	if got := l.Quantity.Mul(l.CostPrice).Round(2).String(); got != "1000" {
		t.Fatalf("120 @ %s adds up to %s", l.CostPrice, got)
	}
	if got := lotCost(l.Subtotal, l.Quantity, l.FreeQty, l.CostPrice); !got.Equal(l.CostPrice) {
		t.Fatalf("lot cost %s, list %s", got, l.CostPrice)
	}
	// Spread over free units: within a cent or two of the bill (cents short of it at 2 decimals: 40 here).
	spread := lotCost(l.Subtotal, l.Quantity.Add(l.Quantity), l.Quantity, l.CostPrice)
	if off := spread.Mul(l.Quantity.Add(l.Quantity)).Sub(l.Subtotal).Abs(); off.GreaterThan(decimal.NewFromFloat(0.02)) {
		t.Fatalf("spread %s is %s off", spread, off)
	}
}
