package sales

import (
	"context"
	"testing"

	"karots-suite/internal/features/warranty"

	"github.com/shopspring/decimal"
)

func returnLine(t *testing.T, svc *Service, d *Detail, uid int64, qty, disposition string) {
	t.Helper()
	_, _, err := svc.PartialReturn(context.Background(), d.Sale.ID, PartialReturnInput{
		Lines: []ReturnLineInput{{SaleItemID: d.Items[0].ID, Quantity: qty, Disposition: disposition}}}, uid)
	mustSales(t, err)
}

func TestFrozenReturnNotRestocked(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	pid, uid := replayFixture(t, conn, "200")
	freeze(t, conn, pid, true)
	svc := NewService(conn)
	d := sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "4"})
	returnLine(t, svc, d, uid, "2", "restock")
	eq(t, "stock", onHand(t, conn, pid), 200)
	eq(t, "lots", lotsLeft(t, conn, pid), 200)
}

func TestFrozenReturnDamaged(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "200")
	freeze(t, conn, pid, true)
	svc := NewService(conn)
	d := sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "4"})
	returnLine(t, svc, d, uid, "2", "damage")
	eq(t, "stock", onHand(t, conn, pid), 200)
	var q, cost decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &q, `SELECT COALESCE(SUM(quantity),0) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, pid))
	mustSales(t, conn.GetContext(ctx, &cost, `SELECT COALESCE(SUM(cost),0) FROM stock_movements WHERE product_id = $1 AND type = 'damage'`, pid))
	if n := moves(t, conn, pid, "damage"); n != 1 || !q.Equal(decimal.NewFromInt(-2)) || !cost.IsPositive() {
		t.Fatalf("damage movements %d qty %s cost %s", n, q, cost)
	}
}

func TestReturnFollowsFrozenNow(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	pid, uid := replayFixture(t, conn, "50")
	svc := NewService(conn)
	d := sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "2"})
	eq(t, "after sale", onHand(t, conn, pid), 48)
	freeze(t, conn, pid, true)
	returnLine(t, svc, d, uid, "1", "")
	eq(t, "returned while frozen", onHand(t, conn, pid), 48)

	d2 := sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "2"})
	freeze(t, conn, pid, false)
	returnLine(t, svc, d2, uid, "1", "")
	eq(t, "returned after unfreeze", onHand(t, conn, pid), 49)
}

func TestFrozenWarrantyReplacement(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "200")
	_, err := conn.ExecContext(ctx, `UPDATE products SET warranty_months = 12 WHERE id = $1`, pid)
	mustSales(t, err)
	freeze(t, conn, pid, true)
	d := sell(t, NewService(conn), uid, ItemInput{ProductID: pid, Quantity: "1"})
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM warranty_claims WHERE unit_id IN (SELECT id FROM warranty_units WHERE product_id = $1)`, pid) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM warranty_units WHERE product_id = $1`, pid)                                                   //nolint:errcheck
	})
	_, err = warranty.NewService(conn).RecordReceiptReplacement(ctx, d.Sale.ID, pid, "broken", uid)
	mustSales(t, err)
	eq(t, "stock", onHand(t, conn, pid), 200)
	var cost decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &cost, `SELECT cost FROM stock_movements WHERE product_id = $1 AND type = 'warranty_replacement'`, pid))
	eq(t, "warranty cost", cost, 50)
}
