package stock

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Batches → Merge batches folds lots into the oldest one. The prompt is
// pre-filled from MergeDefaults (weighted cost, earliest expiry, oldest's
// price) and the person may change each.

func lotRow(t *testing.T, ctx context.Context, tx *sqlx.Tx, id int64) Batch {
	t.Helper()
	var b Batch
	must(t, tx.GetContext(ctx, &b, `SELECT * FROM stock_batches WHERE id = $1`, id))
	return b
}

func ymd(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }

func TestMergeDefaults(t *testing.T) {
	lots := []Batch{
		{ID: 2, QtyRemaining: decimal.NewFromInt(10), CostPrice: decimal.NewFromInt(60), SellingPrice: decimal.NewFromInt(120), ExpiryDate: ymd("2027-05-01"), CreatedAt: time.Unix(200, 0)},
		{ID: 1, QtyRemaining: decimal.NewFromInt(4), CostPrice: decimal.NewFromInt(50), SellingPrice: decimal.NewFromInt(100), CreatedAt: time.Unix(100, 0)},
		{ID: 3, QtyRemaining: decimal.NewFromInt(1), CostPrice: decimal.NewFromInt(60), ExpiryDate: ymd("2027-03-12"), CreatedAt: time.Unix(300, 0)},
	}
	kept, cost, exp, price := MergeDefaults(lots)
	if kept != 1 {
		t.Errorf("kept = %d, want the oldest (1)", kept)
	}
	if !cost.Equal(decimal.RequireFromString("57.33")) { // (200+600+60)/15
		t.Errorf("cost = %s, want 57.33", cost)
	}
	if exp == nil || exp.Format(time.DateOnly) != "2027-03-12" {
		t.Errorf("expiry = %v, want the earliest 2027-03-12", exp)
	}
	if !price.Equal(decimal.NewFromInt(100)) {
		t.Errorf("price = %s, want the oldest's 100", price)
	}
}

func TestMergeBatchesFoldsIntoTheOldest(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, lotB := twoPricedLots(t, ctx, tx)
	must(t, setStock(ctx, repo, productID, decimal.NewFromInt(14)))
	_, err = tx.ExecContext(ctx, `UPDATE stock_batches SET expiry_date = '2027-05-01' WHERE id = $1`, lotB)
	must(t, err)

	kept, err := repo.MergeBatches(ctx, MergeInput{ProductID: productID, BatchIDs: []int64{lotB, lotA}, Expiry: "2027-05-01", Price: "100"})
	must(t, err)
	if kept != lotA {
		t.Fatalf("kept %d, want the oldest %d", kept, lotA)
	}
	a, b := lotRow(t, ctx, tx, lotA), lotRow(t, ctx, tx, lotB)
	if !a.QtyRemaining.Equal(decimal.NewFromInt(14)) || !b.QtyRemaining.IsZero() {
		t.Errorf("lots = %s / %s, want 14 / 0", a.QtyRemaining, b.QtyRemaining)
	}
	if !a.QtyReceived.Equal(decimal.NewFromInt(14)) {
		t.Errorf("kept received = %s, want 14 (4 + the 10 folded in)", a.QtyReceived)
	}
	if !a.CostPrice.Equal(decimal.RequireFromString("57.14")) { // blank cost = weighted (200+600)/14
		t.Errorf("cost = %s, want 57.14", a.CostPrice)
	}
	if a.ExpiryDate == nil || a.ExpiryDate.Format(time.DateOnly) != "2027-05-01" || !a.SellingPrice.Equal(decimal.NewFromInt(100)) {
		t.Errorf("kept = exp %v price %s, want 2027-05-01 / 100", a.ExpiryDate, a.SellingPrice)
	}
	if got := stockQty(t, ctx, repo, productID); !got.Equal(decimal.NewFromInt(14)) || !lotSum(t, ctx, repo, productID).Equal(got) {
		t.Errorf("stock %s changed; a merge never changes the count", got)
	}
	// The emptied lot stays a row a return can still restock into.
	ok, err := repo.RestockLot(ctx, &lotB, productID, decimal.NewFromInt(1))
	must(t, err)
	if !ok {
		t.Error("a return could not restock into the emptied lot")
	}
}

func TestMergeBatchesHonoursEditedValues(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, lotB := twoPricedLots(t, ctx, tx)
	_, err = repo.MergeBatches(ctx, MergeInput{ProductID: productID, BatchIDs: []int64{lotA, lotB}, Cost: "55", Expiry: "", Price: ""})
	must(t, err)
	a := lotRow(t, ctx, tx, lotA)
	if !a.CostPrice.Equal(decimal.NewFromInt(55)) || a.ExpiryDate != nil || !a.SellingPrice.IsZero() {
		t.Errorf("kept = cost %s exp %v price %s, want 55 / none / follows the product (0)", a.CostPrice, a.ExpiryDate, a.SellingPrice)
	}
}

func TestMergeBatchesRefusals(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	repo, productID, lotA, lotB := twoPricedLots(t, ctx, tx)
	other := seedProduct(t, ctx, repo, "TEST other product")
	otherLot, err := repo.InsertBatch(ctx, NewBatch{ProductID: other, Quantity: decimal.NewFromInt(1), CostPrice: decimal.NewFromInt(1)})
	must(t, err)
	empty, err := repo.InsertBatch(ctx, NewBatch{ProductID: productID, Quantity: decimal.Zero, CostPrice: decimal.NewFromInt(1)})
	must(t, err)
	for name, ids := range map[string][]int64{
		"one batch":       {lotA},
		"same twice":      {lotA, lotA},
		"another product": {lotA, otherLot},
		"an empty batch":  {lotA, empty},
	} {
		if _, err := repo.MergeBatches(ctx, MergeInput{ProductID: productID, BatchIDs: ids}); err == nil {
			t.Errorf("%s: merged", name)
		}
	}
	_ = lotB
}
