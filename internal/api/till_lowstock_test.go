package api

import (
	"testing"

	"karots-suite/internal/features/products"

	"github.com/shopspring/decimal"
)

// Low stock: at or below the reorder level; no level set (0) or a service never is.
func TestTillProductLowStock(t *testing.T) {
	p := func(stock string, level int, service bool) products.Product {
		return products.Product{StockQty: decimal.RequireFromString(stock), ReorderLevel: level, IsService: service}
	}
	for _, c := range []struct {
		p    products.Product
		want bool
	}{
		{p("5", 5, false), true},
		{p("6", 5, false), false},
		{p("0", 0, false), false},
		{p("1", 5, true), false},
	} {
		if got := tillProduct(c.p).LowStock; got != c.want {
			t.Errorf("stock %s level %d service %v: got %v", c.p.StockQty, c.p.ReorderLevel, c.p.IsService, got)
		}
	}
}
