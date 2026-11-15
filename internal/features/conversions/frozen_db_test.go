package conversions

import (
	"context"
	"github.com/shopspring/decimal"
	"testing"
)

// Pencil Box → Pencil: a frozen Pencil's 10 go to the (uncounted) shelf.
func TestConversionIntoFrozen(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn) // 2 bags → loose, ratio 25
	_, err := f.tx.ExecContext(ctx, `UPDATE products SET stock_frozen = true WHERE id = $1`, f.loose)
	must(t, err)
	must(t, RunTx(ctx, f.tx, f.convID, d("1"), d("10"), f.user))
	var bag, loose, lots decimal.Decimal
	must(t, f.tx.GetContext(ctx, &bag, `SELECT quantity FROM stock WHERE product_id = $1`, f.bag))
	must(t, f.tx.GetContext(ctx, &loose, `SELECT quantity FROM stock WHERE product_id = $1`, f.loose))
	must(t, f.tx.GetContext(ctx, &lots, `SELECT COALESCE(SUM(qty_remaining),0) FROM stock_batches WHERE product_id = $1`, f.loose))
	if !bag.Equal(d("1")) || !loose.IsZero() || !lots.IsZero() {
		t.Fatalf("bag %s loose %s lots %s; want 1, 0, 0", bag, loose, lots)
	}
	var n int
	must(t, f.tx.GetContext(ctx, &n, `SELECT count(*) FROM stock_movements WHERE type = 'conversion' AND product_id IN ($1, $2)`, f.bag, f.loose))
	if n != 2 {
		t.Fatalf("conversion movements %d, want 2", n)
	}
	// Unfrozen: the 10 come into stock.
	_, err = f.tx.ExecContext(ctx, `UPDATE products SET stock_frozen = false WHERE id = $1`, f.loose)
	must(t, err)
	must(t, RunTx(ctx, f.tx, f.convID, d("1"), d("10"), f.user))
	must(t, f.tx.GetContext(ctx, &loose, `SELECT quantity FROM stock WHERE product_id = $1`, f.loose))
	if !loose.Equal(d("10")) {
		t.Fatalf("loose %s, want 10", loose)
	}
}

// Opened, but the pieces were put in storage: they count even while frozen.
func TestConversionIntoFrozenStorage(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	_, err := f.tx.ExecContext(ctx, `UPDATE products SET stock_frozen = true WHERE id = $1`, f.loose)
	must(t, err)
	must(t, runTx(ctx, f.tx, f.convID, d("1"), d("10"), f.user, true))
	var loose, lots decimal.Decimal
	must(t, f.tx.GetContext(ctx, &loose, `SELECT quantity FROM stock WHERE product_id = $1`, f.loose))
	must(t, f.tx.GetContext(ctx, &lots, `SELECT COALESCE(SUM(qty_remaining),0) FROM stock_batches WHERE product_id = $1`, f.loose))
	if !loose.Equal(d("10")) || !lots.Equal(d("10")) {
		t.Fatalf("loose %s lots %s; want 10, 10", loose, lots)
	}
}

// Bag and loose both frozen: nothing to ask, the pieces go out for sale (uncounted).
func TestConversionBothFrozen(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	_, err := f.tx.ExecContext(ctx, `UPDATE products SET stock_frozen = true WHERE id IN ($1, $2)`, f.bag, f.loose)
	must(t, err)
	must(t, runTx(ctx, f.tx, f.convID, d("1"), d("10"), f.user, true))
	var bag, loose decimal.Decimal
	must(t, f.tx.GetContext(ctx, &bag, `SELECT quantity FROM stock WHERE product_id = $1`, f.bag))
	must(t, f.tx.GetContext(ctx, &loose, `SELECT quantity FROM stock WHERE product_id = $1`, f.loose))
	if !bag.Equal(d("1")) || !loose.IsZero() {
		t.Fatalf("bag %s loose %s; want 1, 0", bag, loose)
	}
}
