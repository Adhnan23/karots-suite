package sales

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// A service line can be returned (a spoilt copy job): the customer is refunded
// like any line, nothing goes back on a shelf, and what the line consumed is
// booked as a loss — returning it takes its COGS off the P&L with the revenue.
func TestServiceLineReturnRefundsAndBooksMaterialsAsLoss(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	var categoryID, unitID, cashierID, sinceSaleID, serviceID, paperID int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &sinceSaleID, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &serviceID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('TEST return copy service', $1, $2, 0, 5, true) RETURNING id`, categoryID, unitID))
	mustSales(t, conn.GetContext(ctx, &paperID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST return paper', $1, $2, 2, 0) RETURNING id`, categoryID, unitID))
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM sale_returns WHERE sale_id > $1`, sinceSaleID)                    //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, sinceSaleID)                                //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id IN ($1,$2)`, serviceID, paperID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id IN ($1,$2)`, serviceID, paperID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id IN ($1,$2)`, serviceID, paperID)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1,$2)`, serviceID, paperID)                //nolint:errcheck
	}()

	svc := NewService(conn)
	// 4 copies at 5 each, 4 sheets of paper at cost 2 (found at till: count was 0).
	d, err := svc.Create(ctx, CreateInput{
		SaleType: "retail",
		Items: []ItemInput{{
			ProductID: serviceID, Quantity: "4", PriceOverride: "5", AllowOversell: true,
			Components: []ServiceComponent{{ProductID: paperID, Quantity: "4"}},
		}},
		Payments: []PaymentInput{{Method: "cash", Amount: "20"}},
	}, cashierID)
	mustSales(t, err)
	item := d.Items[0]
	if !item.CostPrice.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("line cost %s, want 2 per copy", item.CostPrice)
	}

	paper := func() decimal.Decimal {
		var q decimal.Decimal
		mustSales(t, conn.GetContext(ctx, &q, `SELECT COALESCE(quantity,0) FROM stock WHERE product_id = $1`, paperID))
		return q
	}
	losses := func() decimal.Decimal {
		var v decimal.Decimal
		mustSales(t, conn.GetContext(ctx, &v, `
			SELECT COALESCE(SUM(cost),0) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, serviceID))
		return v
	}
	before := paper()

	// Line return of 2 copies: refund 10, loss 4, paper untouched.
	_, cash, err := svc.PartialReturn(ctx, d.Sale.ID, PartialReturnInput{
		Lines: []ReturnLineInput{{SaleItemID: item.ID, Quantity: "2", Disposition: "damage"}},
	}, cashierID)
	mustSales(t, err)
	if !cash.Equal(decimal.NewFromInt(10)) {
		t.Errorf("refund %s, want 10", cash)
	}
	if got := losses(); !got.Equal(decimal.NewFromInt(4)) {
		t.Errorf("losses %s, want 4", got)
	}
	if got := paper(); !got.Equal(before) {
		t.Errorf("paper %s after return, want %s (nothing restocked)", got, before)
	}

	// Whole-sale return no longer refuses a sale with a service line.
	_, cash, _, err = func() (*Detail, decimal.Decimal, int64, error) {
		tx, err := conn.BeginTxx(ctx, nil)
		if err != nil {
			return nil, decimal.Zero, 0, err
		}
		d, c, id, err := svc.ReturnTx(ctx, tx, d.Sale.ID, cashierID)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return nil, decimal.Zero, 0, err
		}
		return d, c, id, tx.Commit()
	}()
	mustSales(t, err)
	if !cash.Equal(decimal.NewFromInt(10)) {
		t.Errorf("whole-sale refund %s, want the last 10", cash)
	}
	if got := losses(); !got.Equal(decimal.NewFromInt(8)) {
		t.Errorf("losses %s, want all 8 of the paper", got)
	}
	var status string
	mustSales(t, conn.GetContext(ctx, &status, `SELECT status FROM sales WHERE id = $1`, d.Sale.ID))
	if status != "returned" {
		t.Errorf("status %q, want returned", status)
	}
}
