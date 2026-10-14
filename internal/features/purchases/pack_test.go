package purchases

import "testing"

// 3 cards @ 250 with 1 free card and Rs 10 off each card: the supplier is owed
// 750 − 30 = 720 exactly; stock comes in as 48 tablets at 20.83 each.
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
	if l.Quantity.String() != "36" || l.FreeQty.String() != "12" || l.CostPrice.String() != "20.83" || l.DiscountValue.String() != "0.83" {
		t.Fatalf("stored %s + %s free @ %s (disc %s)", l.Quantity, l.FreeQty, l.CostPrice, l.DiscountValue)
	}
	if _, _, err := parseLines([]ItemInput{{ProductID: 1, Quantity: "1", CostPrice: "1", PackSize: "1"}}); err == nil {
		t.Fatal("pack size 1 accepted")
	}
}
