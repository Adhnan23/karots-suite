package conversions

import (
	"context"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/stock"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Shares follow each bag's take, the last absorbs the rounding so the total is
// exact, and every share carries the value its bag gave up.
func TestSplitYieldConservesQtyAndValue(t *testing.T) {
	exp := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	no := "B1"
	lots := []stock.ConsumedLot{
		{BatchID: 1, Qty: d("1"), Cost: d("2500"), Expiry: &exp, BatchNo: &no},
		{BatchID: 2, Qty: d("2"), Cost: d("2600")},
	}
	got := splitYield(lots, d("3"), d("73.9"))
	if len(got) != 2 {
		t.Fatalf("got %d batches, want 2", len(got))
	}
	sumQty, sumValue := decimal.Zero, decimal.Zero
	for _, b := range got {
		sumQty = sumQty.Add(b.Quantity)
		sumValue = sumValue.Add(b.Quantity.Mul(b.CostPrice))
	}
	if !sumQty.Equal(d("73.9")) {
		t.Errorf("Σ qty = %s, want 73.9 exactly", sumQty)
	}
	// cost_price is 2 dp per unit, so each batch may be off by half a cent a unit.
	if want := d("7700"); sumValue.Sub(want).Abs().GreaterThan(d("0.005").Mul(d("73.9"))) {
		t.Errorf("Σ value = %s, want 7700 ± 0.37", sumValue)
	}
	if got[0].ExpiryDate == nil || !got[0].ExpiryDate.Equal(exp) || got[0].BatchNo == nil || *got[0].BatchNo != no {
		t.Errorf("first batch lost the bag's expiry / batch no: %+v", got[0])
	}
	if !got[0].Quantity.Equal(d("24.633")) {
		t.Errorf("first share = %s, want 24.633 (1/3 of 73.9)", got[0].Quantity)
	}
}

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

type runFixture struct {
	tx           *sqlx.Tx
	bag, loose   int64
	convID, user int64
}

// bagAndLoose makes a 25 kg bag product with two bags in stock (different
// expiry, batch no and cost) and a loose product, joined by a ratio-25 conversion.
func bagAndLoose(t *testing.T, ctx context.Context, conn *sqlx.DB) runFixture {
	t.Helper()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	t.Cleanup(func() { tx.Rollback() }) //nolint:errcheck
	f := runFixture{tx: tx}
	var categoryID, unitID int64
	must(t, tx.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	must(t, tx.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, tx.GetContext(ctx, &f.user, `SELECT id FROM users LIMIT 1`))
	for _, p := range []struct {
		id   *int64
		name string
	}{{&f.bag, "TEST rice bag"}, {&f.loose, "TEST rice loose"}} {
		must(t, tx.GetContext(ctx, p.id, `
			INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
			VALUES ($1, $2, $3, 2500, 3000) RETURNING id`, p.name, categoryID, unitID))
	}
	stk := stock.NewRepository(tx)
	soon := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC)
	a, b := "A", "B"
	for _, nb := range []stock.NewBatch{
		{ProductID: f.bag, Quantity: d("1"), CostPrice: d("2500"), ExpiryDate: &soon, BatchNo: &a},
		{ProductID: f.bag, Quantity: d("1"), CostPrice: d("2600"), ExpiryDate: &later, BatchNo: &b},
	} {
		_, err := stk.InsertBatch(ctx, nb)
		must(t, err)
	}
	must(t, stk.Increment(ctx, f.bag, d("2")))
	must(t, tx.GetContext(ctx, &f.convID, `
		INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 25) RETURNING id`, f.bag, f.loose))
	return f
}

type lotRow struct {
	Qty     decimal.Decimal `db:"qty_remaining"`
	Cost    decimal.Decimal `db:"cost_price"`
	Expiry  *time.Time      `db:"expiry_date"`
	BatchNo *string         `db:"batch_no"`
}

func looseLots(t *testing.T, ctx context.Context, f runFixture) ([]lotRow, decimal.Decimal) {
	t.Helper()
	var rows []lotRow
	must(t, f.tx.SelectContext(ctx, &rows, `SELECT qty_remaining, cost_price, expiry_date, batch_no
		FROM stock_batches WHERE product_id = $1 ORDER BY expiry_date NULLS LAST, id`, f.loose))
	var mirror decimal.Decimal
	must(t, f.tx.GetContext(ctx, &mirror, `SELECT quantity FROM stock WHERE product_id = $1`, f.loose))
	return rows, mirror
}

// Focus 3: a blank yield runs exactly as before — fromQty × ratio at the bag's cost.
func TestRunBlankYieldIsRatioTimesQty(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	must(t, RunTx(ctx, f.tx, f.convID, d("1"), decimal.Zero, f.user))
	lots, mirror := looseLots(t, ctx, f)
	if len(lots) != 1 || !lots[0].Qty.Equal(d("25")) || !lots[0].Cost.Equal(d("100")) || !mirror.Equal(d("25")) {
		t.Fatalf("lots %+v mirror %s, want one 25 @ 100", lots, mirror)
	}
	if lots[0].Expiry == nil || *lots[0].BatchNo != "A" {
		t.Errorf("loose stock lost the bag's expiry / batch no: %+v", lots[0])
	}
}

// A bag that weighs out 24.6 kg puts its whole cost on those 24.6 kg.
func TestRunActualYieldRaisesUnitCost(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	must(t, RunTx(ctx, f.tx, f.convID, d("1"), d("24.6"), f.user))
	lots, mirror := looseLots(t, ctx, f)
	if len(lots) != 1 || !lots[0].Qty.Equal(d("24.6")) || !mirror.Equal(d("24.6")) {
		t.Fatalf("lots %+v mirror %s, want 24.6", lots, mirror)
	}
	if !lots[0].Cost.Equal(d("101.63")) { // 2500 / 24.6
		t.Errorf("unit cost = %s, want 101.63", lots[0].Cost)
	}
	var toQty decimal.Decimal
	must(t, f.tx.GetContext(ctx, &toQty, `SELECT to_qty FROM conversion_runs WHERE conversion_id = $1`, f.convID))
	if !toQty.Equal(d("24.6")) {
		t.Errorf("run logged to_qty %s, want 24.6", toQty)
	}
}

// Focus 1: two bags → two loose batches, each with its own expiry, batch no and cost.
func TestRunOverTwoBagsKeepsEachBag(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	must(t, RunTx(ctx, f.tx, f.convID, d("2"), d("49.3"), f.user))
	lots, mirror := looseLots(t, ctx, f)
	if len(lots) != 2 || !mirror.Equal(d("49.3")) {
		t.Fatalf("lots %+v mirror %s, want 2 batches totalling 49.3", lots, mirror)
	}
	if *lots[0].BatchNo != "A" || *lots[1].BatchNo != "B" || lots[0].Expiry == nil || lots[1].Expiry == nil {
		t.Errorf("batches did not keep their bags: %+v", lots)
	}
	value := lots[0].Qty.Mul(lots[0].Cost).Add(lots[1].Qty.Mul(lots[1].Cost))
	if value.Sub(d("5100")).Abs().GreaterThan(d("0.005").Mul(d("49.3"))) {
		t.Errorf("Σ value = %s, want 5100 ± half a cent a kg", value)
	}
	if !lots[0].Qty.Add(lots[1].Qty).Equal(d("49.3")) {
		t.Errorf("Σ qty ≠ 49.3")
	}
}

func TestRunRefusesANegativeYield(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	if err := RunTx(ctx, f.tx, f.convID, d("1"), d("-1"), f.user); err == nil {
		t.Fatal("a negative yield ran")
	}
}
