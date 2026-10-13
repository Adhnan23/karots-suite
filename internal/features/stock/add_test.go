package stock

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Intake used to open a new lot for every restock, so a weekly item grew a list
// of identical batches. AddStock tops up a named batch instead, or opens one
// with its own cost / price / expiry (Batches → + Add batch, stock-take + batch).

func stockQty(t *testing.T, ctx context.Context, repo *Repository, productID int64) decimal.Decimal {
	t.Helper()
	q, err := repo.GetQuantity(ctx, productID)
	must(t, err)
	return q
}

func lotSum(t *testing.T, ctx context.Context, repo *Repository, productID int64) decimal.Decimal {
	t.Helper()
	var q decimal.Decimal
	must(t, repo.q.GetContext(ctx, &q, `SELECT COALESCE(SUM(qty_remaining),0) FROM stock_batches WHERE product_id = $1`, productID))
	return q
}

func TestAddStockOpensABatchWithItsOwnCostPriceAndExpiry(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, _, _ := twoPricedLots(t, ctx, tx)
	must(t, setStock(ctx, repo, productID, decimal.NewFromInt(14)))

	id, err := repo.AddStock(ctx, AddStockInput{ProductID: productID, Qty: "5", Cost: "70", Price: "130", Expiry: "2027-03-12", Note: "test"}, 1)
	must(t, err)
	var b Batch
	must(t, tx.GetContext(ctx, &b, `SELECT * FROM stock_batches WHERE id = $1`, id))
	if !b.QtyRemaining.Equal(decimal.NewFromInt(5)) || !b.CostPrice.Equal(decimal.NewFromInt(70)) || !b.SellingPrice.Equal(decimal.NewFromInt(130)) {
		t.Errorf("new batch = %s @ cost %s price %s, want 5 @ 70 / 130", b.QtyRemaining, b.CostPrice, b.SellingPrice)
	}
	if b.ExpiryDate == nil || b.ExpiryDate.Format(time.DateOnly) != "2027-03-12" {
		t.Errorf("expiry = %v, want 2027-03-12", b.ExpiryDate)
	}
	if got := stockQty(t, ctx, repo, productID); !got.Equal(decimal.NewFromInt(19)) {
		t.Errorf("stock = %s, want 19", got)
	}
	if got := movementCost(t, ctx, repo, productID, MoveAdjust); !got.Equal(decimal.NewFromInt(350)) {
		t.Errorf("movement value = %s, want 350 (5 x 70)", got)
	}
}

func TestAddStockTopsUpTheNamedBatch(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, _ := twoPricedLots(t, ctx, tx)
	must(t, setStock(ctx, repo, productID, decimal.NewFromInt(14)))
	before := countLots(t, ctx, repo, productID)

	id, err := repo.AddStock(ctx, AddStockInput{ProductID: productID, BatchID: lotA, Qty: "3"}, 1)
	must(t, err)
	if id != lotA || countLots(t, ctx, repo, productID) != before {
		t.Fatal("a top-up must not open a new batch")
	}
	var recv, rem decimal.Decimal
	must(t, tx.QueryRowxContext(ctx, `SELECT qty_received, qty_remaining FROM stock_batches WHERE id = $1`, lotA).Scan(&recv, &rem))
	if !rem.Equal(decimal.NewFromInt(7)) || !recv.Equal(decimal.NewFromInt(7)) {
		t.Errorf("lot A = %s of %s, want 7 of 7 (received goes up too)", rem, recv)
	}
	if got, sum := stockQty(t, ctx, repo, productID), lotSum(t, ctx, repo, productID); !got.Equal(sum) || !got.Equal(decimal.NewFromInt(17)) {
		t.Errorf("stock %s, lots %s, want both 17", got, sum)
	}
}

func TestAddStockRefusesAnotherProductsBatchAndNoQty(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, _ := twoPricedLots(t, ctx, tx)
	other := seedProduct(t, ctx, repo, "TEST other product")
	if _, err := repo.AddStock(ctx, AddStockInput{ProductID: other, BatchID: lotA, Qty: "1"}, 1); err == nil {
		t.Error("topped up another product's batch")
	}
	if _, err := repo.AddStock(ctx, AddStockInput{ProductID: productID, Qty: "0"}, 1); err == nil {
		t.Error("accepted a zero quantity")
	}
	if _, err := repo.AddStock(ctx, AddStockInput{ProductID: productID, Qty: "1", Expiry: "12/03/2027"}, 1); err == nil {
		t.Error("accepted an expiry that isn't YYYY-MM-DD")
	}
}
