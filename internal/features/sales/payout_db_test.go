package sales

import (
	"context"
	"testing"
	"time"
)

// A payout line is money the shop hands the customer (a cash out): the
// sale may total below zero, the difference leaves the drawer as change, the
// bill discount stays on the goods, and the payout can't be returned.
func TestPayoutLineGivesCashThroughTheSale(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	var categoryID, unitID, cashierID, goods, payout, plain int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mk := func(name string, passThrough bool) int64 {
		var id int64
		mustSales(t, conn.GetContext(ctx, &id, `
			INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service, pass_through)
			VALUES ($1, $2, $3, 0, 0, true, $4) RETURNING id`, name, categoryID, unitID, passThrough))
		return id
	}
	goods, payout, plain = mk("TEST payout goods", false), mk("TEST payout cash out", true), mk("TEST payout plain", false)
	defer conn.ExecContext(ctx, `DELETE FROM products WHERE id = ANY($1)`, []int64{goods, payout, plain}) //nolint:errcheck

	svc := NewService(conn)
	since := time.Now().Add(-time.Second)
	before, err := svc.CashCollectedSince(ctx, cashierID, since)
	mustSales(t, err)

	in := CreateInput{SaleType: "retail", Discount: "100", Items: []ItemInput{
		{ProductID: goods, Quantity: "1", PriceOverride: "1000"},
		{ProductID: payout, Quantity: "1", PriceOverride: "5000", Payout: true},
	}}
	d, err := svc.Create(ctx, in, cashierID)
	mustSales(t, err)
	defer conn.ExecContext(ctx, `DELETE FROM sales WHERE id = $1`, d.Sale.ID) //nolint:errcheck
	if d.Sale.Total.String() != "-4100" || d.Sale.ChangeGiven.String() != "4100" || d.PaidFactor.StringFixed(2) != "0.90" {
		t.Fatalf("total %s change %s factor %s, want -4100 / 4100 / 0.90", d.Sale.Total, d.Sale.ChangeGiven, d.PaidFactor)
	}
	after, err := svc.CashCollectedSince(ctx, cashierID, since)
	mustSales(t, err)
	if !after.Sub(before).Equal(d.Sale.ChangeGiven.Neg()) {
		t.Fatalf("drawer moved %s, want -4100", after.Sub(before))
	}
	var cashRows int
	mustSales(t, conn.GetContext(ctx, &cashRows, `SELECT count(*) FROM payments WHERE sale_id = $1 AND method = 'cash'`, d.Sale.ID))
	if cashRows != 1 {
		t.Fatalf("a payout sale keeps one cash row (for the by-method totals), got %d", cashRows)
	}

	var payoutItem, goodsItem SaleItem
	for _, it := range d.Items {
		if it.ProductID == payout {
			payoutItem = it
		} else {
			goodsItem = it
		}
	}
	if !payoutItem.ReturnableQty().IsZero() {
		t.Fatalf("a payout line is not returnable")
	}
	_, refund, err := svc.PartialReturn(ctx, d.Sale.ID, PartialReturnInput{Lines: []ReturnLineInput{
		{SaleItemID: goodsItem.ID, Quantity: "1"}}}, cashierID)
	mustSales(t, err)
	if refund.String() != "900" {
		t.Fatalf("goods refund %s, want 900 (bill discount on the goods only)", refund)
	}
	var status string
	mustSales(t, conn.GetContext(ctx, &status, `SELECT status FROM sales WHERE id = $1`, d.Sale.ID))
	if status != "returned" {
		t.Fatalf("all goods back → %s, want returned", status)
	}

	refused := map[string]CreateInput{
		"not pass-through": {SaleType: "retail", Items: []ItemInput{{ProductID: plain, Quantity: "1", PriceOverride: "100", Payout: true}}},
		"two units":        {SaleType: "retail", Items: []ItemInput{{ProductID: payout, Quantity: "2", PriceOverride: "100", Payout: true}}},
		"discounted":       {SaleType: "retail", Items: []ItemInput{{ProductID: payout, Quantity: "1", PriceOverride: "100", Discount: "10", Payout: true}}},
		"card on a payout": {SaleType: "retail", Items: []ItemInput{{ProductID: payout, Quantity: "1", PriceOverride: "100", Payout: true}},
			Payments: []PaymentInput{{Method: "card", Amount: "50", AccountID: 1}}},
	}
	for name, in := range refused {
		if _, err := svc.Create(ctx, in, cashierID); err == nil {
			t.Fatalf("%s: a bad payout was accepted", name)
		}
	}
}
