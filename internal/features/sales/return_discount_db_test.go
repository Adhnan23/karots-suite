package sales

import (
	"context"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// A return refunds what the customer actually PAID for the goods. The refund
// used to be the line net ÷ qty — the item discount was honoured but the bill
// discount was not, so a bill-discounted sale refunded more than was paid, and
// the refund slip, the drawer and the customer's credit all followed it.
//
// Sale (commits, like the other sales DB tests; cleaned up after):
//
//	A: 3 @ 100, 10% item discount → net 270
//	B: 2 @  50                    → net 100
//	bill: Rs 10 off the 370       → paid 360   (each Rs of net paid 360/370)
//
// A's line was paid 270×360/370 = 262.70, B's 100×360/370 = 97.30.
// discountedSale rings the sale above and returns the service, the sale and the
// sale-item id per product. Everything it creates is removed by t.Cleanup.
func discountedSale(t *testing.T, ctx context.Context, conn *sqlx.DB) (*Service, *Detail, int64, int64, map[int64]int64) {
	t.Helper()

	var categoryID, unitID, cashierID, sinceSaleID, prodA, prodB int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &sinceSaleID, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &prodA, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST return disc A', $1, $2, 60, 100) RETURNING id`, categoryID, unitID))
	mustSales(t, conn.GetContext(ctx, &prodB, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST return disc B', $1, $2, 30, 50) RETURNING id`, categoryID, unitID))
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM sale_returns WHERE sale_id > $1`, sinceSaleID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, sinceSaleID)             //nolint:errcheck
		for _, p := range []int64{prodA, prodB} {
			conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, p) //nolint:errcheck
			conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, p)   //nolint:errcheck
			conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, p)           //nolint:errcheck
			conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, p)                //nolint:errcheck
		}
	})

	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{
		SaleType: "retail", Discount: "10", DiscountType: "fixed",
		Items: []ItemInput{
			{ProductID: prodA, Quantity: "3", Discount: "10", DiscountType: "percent", AllowOversell: true},
			{ProductID: prodB, Quantity: "2", AllowOversell: true},
		},
		Payments: []PaymentInput{{Method: "cash", Amount: "360"}},
	}, cashierID)
	mustSales(t, err)
	if !d.Sale.Total.Equal(decimal.NewFromInt(360)) {
		t.Fatalf("sale total %s, want 360", d.Sale.Total)
	}
	itemOf := map[int64]int64{}
	for _, it := range d.Items {
		itemOf[it.ProductID] = it.ID
	}
	return svc, d, prodA, prodB, itemOf
}

func TestReturnRefundsWhatWasPaidAfterBillDiscount(t *testing.T) {
	conn := salesTestDB(t)
	t.Cleanup(func() { conn.Close() }) // registered first, so it runs after the helper's cleanup
	ctx := context.Background()
	svc, d, prodA, prodB, itemOf := discountedSale(t, ctx, conn)
	cashierID := d.Sale.CashierID
	ret := func(productID int64, qty string) decimal.Decimal {
		t.Helper()
		_, cash, err := svc.PartialReturn(ctx, d.Sale.ID, PartialReturnInput{
			Lines: []ReturnLineInput{{SaleItemID: itemOf[productID], Quantity: qty}},
		}, cashierID)
		mustSales(t, err)
		return cash
	}

	// 1 of A: 90 net × 360/370 = 87.57 — NOT the 90 the old math refunded.
	if got := ret(prodA, "1"); !got.Equal(decimal.RequireFromString("87.57")) {
		t.Errorf("refund for 1 of A = %s, want 87.57", got)
	}
	// The rest of A takes exactly what's left of A's 262.70 (no cent drift).
	if got := ret(prodA, "2"); !got.Equal(decimal.RequireFromString("175.13")) {
		t.Errorf("refund for last 2 of A = %s, want 175.13", got)
	}
	if got := ret(prodB, "2"); !got.Equal(decimal.RequireFromString("97.30")) {
		t.Errorf("refund for B = %s, want 97.30", got)
	}

	// Everything back = exactly what was paid.
	var refunded decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &refunded,
		`SELECT SUM(refund_amount) FROM sale_returns WHERE sale_id = $1`, d.Sale.ID))
	if !refunded.Equal(decimal.NewFromInt(360)) {
		t.Errorf("refunded in total %s, want the 360 paid", refunded)
	}
	// The sales list values the returns the same way the refunds were paid.
	var retVal decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &retVal,
		`SELECT ret_val FROM sales s, LATERAL (`+saleProfitAgg+`) a WHERE s.id = $1`, d.Sale.ID))
	if !retVal.Round(2).Equal(decimal.NewFromInt(360)) {
		t.Errorf("sales list return value %s, want 360", retVal)
	}
}

// The whole-sale return records its refund like a line return does: what was
// paid (360), not the line nets (370), and not nothing — it used to record no
// return at all, so it never reached the refund slip or the reports.
func TestWholeSaleReturnRecordsWhatWasPaid(t *testing.T) {
	conn := salesTestDB(t)
	t.Cleanup(func() { conn.Close() }) // registered first, so it runs after the helper's cleanup
	ctx := context.Background()
	svc, d, _, _, _ := discountedSale(t, ctx, conn)

	got, err := svc.Return(ctx, d.Sale.ID, d.Sale.CashierID)
	mustSales(t, err)
	if got.Sale.Status != "returned" {
		t.Errorf("status %q, want returned", got.Sale.Status)
	}
	var refunded decimal.Decimal
	var lines int
	mustSales(t, conn.GetContext(ctx, &refunded,
		`SELECT COALESCE(SUM(refund_amount), 0) FROM sale_returns WHERE sale_id = $1`, d.Sale.ID))
	mustSales(t, conn.GetContext(ctx, &lines, `
		SELECT count(*) FROM sale_return_items sri JOIN sale_returns sr ON sr.id = sri.return_id
		WHERE sr.sale_id = $1`, d.Sale.ID))
	if !refunded.Equal(decimal.NewFromInt(360)) || lines != 2 {
		t.Errorf("recorded refund %s over %d lines, want 360 over 2", refunded, lines)
	}
	if _, err := svc.Return(ctx, d.Sale.ID, d.Sale.CashierID); err == nil {
		t.Error("a second whole-sale return was accepted")
	}
}

// Returns on a part-paid credit sale clear only the debt that's left. Every
// return used to take up to total − paid off the account, because paid_amount
// never changes: 3 @ 100, 100 cash + 200 on account, three returns of 1 took
// 300 off a 200 debt and refunded no cash.
func TestReturnsClearOnlyTheDebtLeft(t *testing.T) {
	conn := salesTestDB(t)
	t.Cleanup(func() { conn.Close() })
	ctx := context.Background()

	var categoryID, unitID, cashierID, sinceSaleID, prod, cust int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &sinceSaleID, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &prod, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST return credit', $1, $2, 60, 100) RETURNING id`, categoryID, unitID))
	mustSales(t, conn.GetContext(ctx, &cust, `
		INSERT INTO customers (name, phone, credit_limit) VALUES ('TEST return credit', '0700000999', 1000) RETURNING id`))
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM sale_returns WHERE sale_id > $1`, sinceSaleID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, sinceSaleID)               //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, prod)    //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, prod)      //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, prod)              //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, prod)                   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM customer_payments WHERE customer_id = $1`, cust) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, cust)                  //nolint:errcheck
	})

	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{
		CustomerID: &cust, SaleType: "retail",
		Items:    []ItemInput{{ProductID: prod, Quantity: "3", AllowOversell: true}},
		Payments: []PaymentInput{{Method: "cash", Amount: "100"}, {Method: "credit", Amount: "200"}},
	}, cashierID)
	mustSales(t, err)
	item := d.Items[0].ID

	var cash []string
	for range 3 {
		_, c, err := svc.PartialReturn(ctx, d.Sale.ID, PartialReturnInput{
			Lines: []ReturnLineInput{{SaleItemID: item, Quantity: "1"}},
		}, cashierID)
		mustSales(t, err)
		cash = append(cash, c.StringFixed(2))
	}
	if want := []string{"0.00", "0.00", "100.00"}; strings.Join(cash, " ") != strings.Join(want, " ") {
		t.Errorf("cash refunds %v, want %v (debt first, then the cash paid)", cash, want)
	}
	var balance decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &balance, `SELECT outstanding_balance FROM customers WHERE id = $1`, cust))
	if !balance.IsZero() {
		t.Errorf("customer balance %s after returning everything, want 0", balance)
	}
}
