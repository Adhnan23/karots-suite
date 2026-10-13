package stock

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// A conversion has to know which bags it opened — their expiry, batch no and
// cost — so the loose stock can carry them on. DepleteFEFOLots says exactly that.
func TestDepleteFEFOLotsReportsEachBatchTaken(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo := NewRepository(tx)
	var categoryID, unitID, productID int64
	must(t, tx.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	must(t, tx.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, tx.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST deplete lots', $1, $2, 60, 120) RETURNING id`, categoryID, unitID))

	soon := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	noA, noB := "A-1", "B-2"
	// Inserted later-expiry first, so FEFO order differs from insert order.
	lotLate, err := repo.InsertBatch(ctx, NewBatch{ProductID: productID, Quantity: d("5"),
		CostPrice: d("70"), ExpiryDate: &later, BatchNo: &noB})
	must(t, err)
	lotSoon, err := repo.InsertBatch(ctx, NewBatch{ProductID: productID, Quantity: d("2"),
		CostPrice: d("50"), ExpiryDate: &soon, BatchNo: &noA})
	must(t, err)

	lots, avg, err := repo.DepleteFEFOLots(ctx, productID, d("3"))
	must(t, err)
	if len(lots) != 2 {
		t.Fatalf("got %d lots, want 2", len(lots))
	}
	first, second := lots[0], lots[1]
	if first.BatchID != lotSoon || !first.Qty.Equal(d("2")) || !first.Cost.Equal(d("50")) ||
		first.Expiry == nil || !first.Expiry.Equal(soon) || first.BatchNo == nil || *first.BatchNo != noA {
		t.Errorf("first lot = %+v, want the soon batch, 2 @ 50, A-1", first)
	}
	if second.BatchID != lotLate || !second.Qty.Equal(d("1")) || !second.Cost.Equal(d("70")) ||
		second.Expiry == nil || !second.Expiry.Equal(later) || *second.BatchNo != noB {
		t.Errorf("second lot = %+v, want the late batch, 1 @ 70, B-2", second)
	}
	// (2×50 + 1×70) / 3 = 56.67
	if !avg.Equal(d("56.67")) {
		t.Errorf("avg cost = %s, want 56.67", avg)
	}

	// DepleteFEFO is the same walk, returning only the cost.
	cost, err := repo.DepleteFEFO(ctx, productID, d("1"))
	must(t, err)
	if !cost.Equal(d("70")) {
		t.Errorf("DepleteFEFO cost = %s, want 70", cost)
	}
}

// A zero-cost lot carries the product's cost, so a conversion never makes
// free loose stock out of a bag whose cost box was left blank.
func TestDepleteFEFOLotsRescuesAZeroCostLot(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, _ := twoPricedLots(t, ctx, tx)
	must(t, tx.QueryRowxContext(ctx, `UPDATE stock_batches SET cost_price = 0 WHERE id = $1 RETURNING id`, lotA).Scan(&lotA))
	lots, _, err := repo.DepleteFEFOLots(ctx, productID, d("1"))
	must(t, err)
	if len(lots) != 1 || !lots[0].Cost.Equal(decimal.NewFromInt(60)) {
		t.Errorf("lots = %+v, want one lot costed at the product's 60", lots)
	}
}
