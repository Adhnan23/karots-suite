package sales

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Stock freeze: a frozen product sells without touching its stock or lots.

func freeze(t *testing.T, conn *sqlx.DB, id int64, on bool) {
	t.Helper()
	_, err := conn.ExecContext(context.Background(), `UPDATE products SET stock_frozen = $2 WHERE id = $1`, id, on)
	mustSales(t, err)
	// Leave nothing frozen behind (a fixture whose sale can't be deleted stays).
	t.Cleanup(func() { conn.ExecContext(context.Background(), `UPDATE products SET stock_frozen = false WHERE id = $1`, id) }) //nolint:errcheck
}

func onHand(t *testing.T, conn *sqlx.DB, id int64) decimal.Decimal {
	t.Helper()
	var q decimal.Decimal
	mustSales(t, conn.GetContext(context.Background(), &q, `SELECT COALESCE((SELECT quantity FROM stock WHERE product_id = $1), 0)`, id))
	return q
}

func lotsLeft(t *testing.T, conn *sqlx.DB, id int64) decimal.Decimal {
	t.Helper()
	var q decimal.Decimal
	mustSales(t, conn.GetContext(context.Background(), &q, `SELECT COALESCE(SUM(qty_remaining), 0) FROM stock_batches WHERE product_id = $1`, id))
	return q
}

func moves(t *testing.T, conn *sqlx.DB, id int64, typ string) int {
	t.Helper()
	var n int
	mustSales(t, conn.GetContext(context.Background(), &n, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND type = $2::stock_movement_type`, id, typ))
	return n
}

func sell(t *testing.T, svc *Service, uid int64, items ...ItemInput) *Detail {
	t.Helper()
	total := decimal.Zero
	for _, it := range items {
		total = total.Add(decimal.RequireFromString(it.Quantity).Mul(decimal.NewFromInt(120)))
	}
	d, err := svc.Create(context.Background(), CreateInput{SaleType: "retail", Items: items,
		Payments: []PaymentInput{{Method: "cash", Amount: total.String()}}}, uid)
	mustSales(t, err)
	return d
}

func eq(t *testing.T, what string, got decimal.Decimal, want int64) {
	t.Helper()
	if !got.Equal(decimal.NewFromInt(want)) {
		t.Fatalf("%s = %s, want %d", what, got, want)
	}
}

func TestFrozenSaleKeepsStock(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	pid, uid := replayFixture(t, conn, "200")
	freeze(t, conn, pid, true)
	d := sell(t, NewService(conn), uid, ItemInput{ProductID: pid, Quantity: "4"})
	eq(t, "stock", onHand(t, conn, pid), 200)
	eq(t, "lots", lotsLeft(t, conn, pid), 200)
	if n := moves(t, conn, pid, "sale"); n != 0 {
		t.Fatalf("sale movements = %d", n)
	}
	if n := moves(t, conn, pid, "adjust"); n != 0 {
		t.Fatalf("found movements = %d", n)
	}
	if fs := flagKinds(t, NewService(conn), d.Sale.ID); len(fs) != 0 {
		t.Fatalf("flags %+v", fs)
	}
	eq(t, "line cost", d.Items[0].CostPrice, 50)
}

func TestFrozenSaleAtZeroNeverAsks(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	pid, uid := replayFixture(t, conn, "0")
	freeze(t, conn, pid, true)
	sell(t, NewService(conn), uid, ItemInput{ProductID: pid, Quantity: "3"})
	eq(t, "stock", onHand(t, conn, pid), 0)
	if n := moves(t, conn, pid, "adjust"); n != 0 {
		t.Fatalf("found movements = %d", n)
	}
}

func TestFrozenMixedCart(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	a, uid := replayFixture(t, conn, "200")
	b, _ := replayFixture(t, conn, "50")
	freeze(t, conn, a, true)
	sell(t, NewService(conn), uid, ItemInput{ProductID: a, Quantity: "4"}, ItemInput{ProductID: b, Quantity: "5"})
	eq(t, "A", onHand(t, conn, a), 200)
	eq(t, "B", onHand(t, conn, b), 45)
	eq(t, "B lots", lotsLeft(t, conn, b), 45)
	if n := moves(t, conn, b, "sale"); n != 1 {
		t.Fatalf("B sale movements = %d", n)
	}
}

func TestFrozenServiceComponent(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	paper, uid := replayFixture(t, conn, "100")
	svcID, _ := replayFixture(t, conn, "0")
	_, err := conn.ExecContext(ctx, `UPDATE products SET is_service = true WHERE id = $1`, svcID)
	mustSales(t, err)
	svc := NewService(conn)
	job := ItemInput{ProductID: svcID, Quantity: "1", Components: []ServiceComponent{{ProductID: paper, Quantity: "2"}}}
	freeze(t, conn, paper, true)
	d := sell(t, svc, uid, job)
	eq(t, "frozen paper", onHand(t, conn, paper), 100)
	eq(t, "frozen paper lots", lotsLeft(t, conn, paper), 100)
	eq(t, "job cost", d.Items[0].CostPrice, 100) // 2 sheets × cost 50
	freeze(t, conn, paper, false)
	sell(t, svc, uid, job)
	eq(t, "paper", onHand(t, conn, paper), 98)
}

func TestFrozenPickedLot(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	var lot int64
	mustSales(t, conn.GetContext(ctx, &lot, `
		INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price) VALUES ($1, 5, 5, 60, 150) RETURNING id`, pid))
	_, err := conn.ExecContext(ctx, `UPDATE stock SET quantity = 15 WHERE product_id = $1`, pid)
	mustSales(t, err)
	freeze(t, conn, pid, true)
	d, err := NewService(conn).Create(ctx, CreateInput{SaleType: "retail",
		Items:    []ItemInput{{ProductID: pid, Quantity: "2", BatchID: lot}},
		Payments: []PaymentInput{{Method: "cash", Amount: "300"}}}, uid)
	mustSales(t, err)
	eq(t, "price", d.Items[0].UnitPrice, 150)
	var left decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &left, `SELECT qty_remaining FROM stock_batches WHERE id = $1`, lot))
	eq(t, "picked lot", left, 5)
	eq(t, "stock", onHand(t, conn, pid), 15)
}

func TestReplayAfterUnfreeze(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	pid, uid := replayFixture(t, conn, "10")
	at := time.Now().Add(-time.Hour)
	// Rung offline while frozen; by replay time it was unfrozen: the server's state now wins.
	_, err := NewService(conn).Create(context.Background(), CreateInput{SaleType: "retail", Review: true,
		ClientRef: "TEST-frz-" + time.Now().Format("150405.000"), RungAt: &at,
		Items:    []ItemInput{{ProductID: pid, Quantity: "3"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "360"}}}, uid)
	mustSales(t, err)
	eq(t, "stock", onHand(t, conn, pid), 7)
}

// A frozen product's picked lot only sets the price: selling more than the lot
// holds is fine (the shelf isn't counted) and the lot stays as it was.
func TestFrozenPickedLotNeverShort(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	var lot int64
	mustSales(t, conn.GetContext(ctx, &lot, `
		INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price) VALUES ($1, 2, 2, 60, 150) RETURNING id`, pid))
	freeze(t, conn, pid, true)
	d, err := NewService(conn).Create(ctx, CreateInput{SaleType: "retail",
		Items:    []ItemInput{{ProductID: pid, Quantity: "5", BatchID: lot}},
		Payments: []PaymentInput{{Method: "cash", Amount: "750"}}}, uid)
	mustSales(t, err)
	eq(t, "price", d.Items[0].UnitPrice, 150)
	var left decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &left, `SELECT qty_remaining FROM stock_batches WHERE id = $1`, lot))
	eq(t, "picked lot", left, 2)
}
