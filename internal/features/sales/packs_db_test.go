package sales

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// 14 tablets of a product sold by the Card ×12 @ 250 (26 a tablet) book as one
// line: 14 units, subtotal 250 + 2 × 26 = 302, label "1 Card + 2". A part
// return refunds its share of what was paid and the rest takes the remainder.
func TestSaleWithPack(t *testing.T) {
	conn := salesTestDB(t)
	t.Cleanup(func() { conn.Close() })
	ctx := context.Background()
	var categoryID, unitID, cashierID, since, prod int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units WHERE NOT allow_decimal LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &since, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &prod, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST pack Panadol', $1, $2, 18, 26) RETURNING id`, categoryID, unitID))
	_, err := conn.ExecContext(ctx, `INSERT INTO product_packs (product_id, name, size, selling_price) VALUES ($1, 'Card', 12, 250)`, prod)
	mustSales(t, err)
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM sale_returns WHERE sale_id > $1`, since)     //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, since)                 //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, prod) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, prod)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, prod)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, prod)                //nolint:errcheck
	})

	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{
		SaleType: "retail",
		Items:    []ItemInput{{ProductID: prod, Quantity: "14", AllowOversell: true}},
		Payments: []PaymentInput{{Method: "cash", Amount: "302"}},
	}, cashierID)
	mustSales(t, err)
	it := d.Items[0]
	if !d.Sale.Total.Equal(decimal.NewFromInt(302)) || !it.Subtotal.Equal(decimal.NewFromInt(302)) ||
		!it.Quantity.Equal(decimal.NewFromInt(14)) || !it.UnitPrice.Equal(decimal.NewFromInt(26)) {
		t.Fatalf("sale total %s, line %s × %s = %s", d.Sale.Total, it.Quantity, it.UnitPrice, it.Subtotal)
	}
	if it.PackLabel == nil || *it.PackLabel != "1 Card + 2" {
		t.Fatalf("pack label %v", it.PackLabel)
	}
	var moved decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &moved, `SELECT COALESCE(SUM(quantity), 0) FROM stock_movements WHERE product_id = $1 AND type = 'sale'`, prod))
	if !moved.Abs().Equal(decimal.NewFromInt(14)) {
		t.Errorf("stock moved %s, want 14 units", moved)
	}

	ret := func(qty string) decimal.Decimal {
		t.Helper()
		_, cash, err := svc.PartialReturn(ctx, d.Sale.ID, PartialReturnInput{
			Lines: []ReturnLineInput{{SaleItemID: it.ID, Quantity: qty}},
		}, cashierID)
		mustSales(t, err)
		return cash
	}
	if got := ret("5"); !got.Equal(decimal.RequireFromString("107.86")) {
		t.Errorf("refund for 5 = %s, want 107.86 (302 × 5/14)", got)
	}
	if got := ret("9"); !got.Equal(decimal.RequireFromString("194.14")) {
		t.Errorf("refund for the last 9 = %s, want 194.14", got)
	}
}
