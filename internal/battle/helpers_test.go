package battle

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"

	"karots-suite/internal/demo/gen"
)

// line / pay build the till's request bodies.
type line = map[string]any
type pay = map[string]any

func cash(amount string) pay { return pay{"method": "cash", "amount": amount} }

// sale is what the counter's sale screen reads (only the parts the tests use).
type sale struct {
	ID         int64  `json:"id"`
	Total      string `json:"total"`
	Status     string `json:"status"`
	OwedOnSale string `json:"owedOnSale"`
	Lines      []struct {
		ID          int64  `json:"id"`
		Quantity    string `json:"quantity"`
		ReturnedQty string `json:"returnedQty"`
		Paid        string `json:"paid"`
		RefundLeft  string `json:"refundLeft"`
	} `json:"lines"`
}

// sell rings a retail sale with a fixed bill discount.
func (s *shop) sell(t testing.TB, c *gen.Client, billOff string, items []line, pays ...pay) saleResult {
	t.Helper()
	var r saleResult
	s.must(t, c.Do("POST", "/api/v1/till/sales", map[string]any{
		"saleType": "retail", "discount": billOff, "discountType": "fixed", "items": items, "payments": pays,
	}, &r))
	return r
}

func (s *shop) sale(t testing.TB, c *gen.Client, id int64) sale {
	t.Helper()
	var v sale
	s.must(t, c.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", id), nil, &v))
	return v
}

// giveBack returns qty of one line at the counter; the cash refunded.
func (s *shop) giveBack(t testing.TB, c *gen.Client, saleID, itemID int64, qty string) decimal.Decimal {
	t.Helper()
	var r struct{ CashRefund string }
	s.must(t, c.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", saleID), map[string]any{
		"lines": []map[string]any{{"saleItemId": itemID, "quantity": qty}}}, &r))
	return dec(t, r.CashRefund)
}

func dec(t testing.TB, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("not a number: %q", s)
	}
	return d
}
