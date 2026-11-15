package stock

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// A frozen product's write-off: from the shelf = an extra comes in, then goes to
// damaged (stock unchanged); from the back = a normal write-off.
func TestFrozenWriteOffShelfAndBack(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck
	repo := NewRepository(tx)
	pid := seedProduct(t, ctx, repo, "TEST frozen write-off")
	var uid int64
	must(t, tx.GetContext(ctx, &uid, `SELECT id FROM users LIMIT 1`))
	_, err = repo.InsertBatch(ctx, NewBatch{ProductID: pid, Quantity: decimal.NewFromInt(200), CostPrice: decimal.NewFromInt(10)})
	must(t, err)
	must(t, repo.Increment(ctx, pid, decimal.NewFromInt(200)))
	_, err = tx.ExecContext(ctx, `UPDATE products SET stock_frozen = true WHERE id = $1`, pid)
	must(t, err)

	check := func(want int64, damages int) {
		t.Helper()
		q, err := repo.GetQuantity(ctx, pid)
		must(t, err)
		var n int
		var cost decimal.Decimal
		must(t, tx.GetContext(ctx, &n, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, pid))
		must(t, tx.GetContext(ctx, &cost, `SELECT COALESCE(SUM(cost),0) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, pid))
		if !q.Equal(decimal.NewFromInt(want)) || n != damages || !cost.Equal(decimal.NewFromInt(int64(10*damages))) {
			t.Fatalf("stock %s damages %d cost %s; want %d, %d", q, n, cost, want, damages)
		}
	}
	_, err = consumeTx(ctx, tx, ConsumeInput{ProductID: pid, Quantity: "1", Reason: "damage", FromShelf: true}, uid)
	must(t, err)
	check(200, 1)
	_, err = consumeTx(ctx, tx, ConsumeInput{ProductID: pid, Quantity: "1", Reason: "damage"}, uid)
	must(t, err)
	check(199, 2)
	// Not frozen: FromShelf is ignored.
	_, err = tx.ExecContext(ctx, `UPDATE products SET stock_frozen = false WHERE id = $1`, pid)
	must(t, err)
	_, err = consumeTx(ctx, tx, ConsumeInput{ProductID: pid, Quantity: "1", Reason: "damage", FromShelf: true}, uid)
	must(t, err)
	check(198, 3)
}
