package sales

import (
	"context"
	"testing"

	"karots-suite/internal/features/stock"
)

// The spec's pencil day: 200 counted and frozen; sales don't touch it; a box of 10
// opened onto the shelf → 190; unfrozen with 13 on the shelf → 203; then sales count.
func TestPencilDay(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "200")
	st, svc := stock.NewService(conn), NewService(conn)
	res, err := st.Freeze(ctx, []int64{pid})
	mustSales(t, err)
	if res.Frozen != 1 || len(res.Refused) != 0 {
		t.Fatalf("freeze %+v", res)
	}
	sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "4"})
	eq(t, "after sale", onHand(t, conn, pid), 200)
	mustSales(t, st.MoveToShelf(ctx, pid, "10", uid))
	eq(t, "opened a box", onHand(t, conn, pid), 190)
	eq(t, "lots", lotsLeft(t, conn, pid), 190)
	if n := moves(t, conn, pid, "to_shelf"); n != 1 {
		t.Fatalf("to_shelf movements %d", n)
	}
	if n := moves(t, conn, pid, "adjust") + moves(t, conn, pid, "damage"); n != 0 {
		t.Fatalf("an adjustment or loss was written: %d", n)
	}
	rows, err := st.FrozenProducts(ctx)
	mustSales(t, err)
	found := false
	for _, r := range rows {
		found = found || r.ID == pid
	}
	if !found {
		t.Fatal("not on the Frozen list")
	}
	mustSales(t, st.Unfreeze(ctx, pid, "13", uid))
	eq(t, "unfrozen", onHand(t, conn, pid), 203)
	eq(t, "unfrozen lots", lotsLeft(t, conn, pid), 203)
	sell(t, svc, uid, ItemInput{ProductID: pid, Quantity: "1"})
	eq(t, "counted sale", onHand(t, conn, pid), 202)
	if n := moves(t, conn, pid, "adjust"); n != 0 {
		t.Fatalf("unfreeze wrote an adjustment")
	}
}

func TestUnfreezeRules(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "200")
	st := stock.NewService(conn)
	if err := st.Unfreeze(ctx, pid, "1", uid); err == nil {
		t.Fatal("unfreeze of a product that isn't frozen")
	}
	if err := st.MoveToShelf(ctx, pid, "1", uid); err == nil {
		t.Fatal("moved to the shelf while not frozen")
	}
	freeze(t, conn, pid, true)
	for _, bad := range []string{"-1", "abc", ""} {
		if err := st.Unfreeze(ctx, pid, bad, uid); err == nil {
			t.Fatalf("shelf count %q accepted", bad)
		}
	}
	mustSales(t, st.Unfreeze(ctx, pid, "0", uid))
	eq(t, "zero on the shelf", onHand(t, conn, pid), 200)
	if err := st.Unfreeze(ctx, pid, "0", uid); err == nil {
		t.Fatal("unfrozen twice")
	}
}

func TestMoveToShelfMoreThanStock(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "190")
	freeze(t, conn, pid, true)
	if err := stock.NewService(conn).MoveToShelf(ctx, pid, "191", uid); err == nil {
		t.Fatal("moved more than the stock")
	}
	eq(t, "stock", onHand(t, conn, pid), 190)
	if n := moves(t, conn, pid, "to_shelf"); n != 0 {
		t.Fatal("movement written")
	}
}

func TestUnfreezeMidSale(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "200")
	freeze(t, conn, pid, true)
	mustSales(t, stock.NewService(conn).Unfreeze(ctx, pid, "2", uid))
	eq(t, "unfrozen", onHand(t, conn, pid), 202)
	sell(t, NewService(conn), uid, ItemInput{ProductID: pid, Quantity: "4"})
	eq(t, "sold", onHand(t, conn, pid), 198)
}

func TestFreezeShop(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	a, _ := replayFixture(t, conn, "5")
	b, _ := replayFixture(t, conn, "5")
	sv, _ := replayFixture(t, conn, "0")
	_, err := conn.ExecContext(ctx, `UPDATE products SET is_service = true WHERE id = $1`, sv)
	mustSales(t, err)
	saved := stock.FreezeRefusals
	stock.FreezeRefusals = func(_ context.Context, ids []int64) (map[int64]string, error) {
		return map[int64]string{b: "counted in full"}, nil
	}
	t.Cleanup(func() {
		stock.FreezeRefusals = saved
		conn.ExecContext(ctx, `UPDATE products SET stock_frozen = false, stock_frozen_at = NULL`) //nolint:errcheck
		conn.ExecContext(ctx, `UPDATE settings SET freeze_new_products = false`)                  //nolint:errcheck
	})
	st := stock.NewService(conn)
	res, err := st.FreezeShop(ctx)
	mustSales(t, err)
	if res.Refused[b] != "counted in full" {
		t.Fatalf("refused %+v", res.Refused)
	}
	var fa, fb, fs, setting bool
	mustSales(t, conn.GetContext(ctx, &fa, `SELECT stock_frozen FROM products WHERE id = $1`, a))
	mustSales(t, conn.GetContext(ctx, &fb, `SELECT stock_frozen FROM products WHERE id = $1`, b))
	mustSales(t, conn.GetContext(ctx, &fs, `SELECT stock_frozen FROM products WHERE id = $1`, sv))
	mustSales(t, conn.GetContext(ctx, &setting, `SELECT freeze_new_products FROM settings WHERE id = 1`))
	if !fa || fb || fs || !setting {
		t.Fatalf("a %v b %v service %v setting %v", fa, fb, fs, setting)
	}
	n, err := st.UnfreezeShop(ctx)
	mustSales(t, err)
	var left int
	mustSales(t, conn.GetContext(ctx, &left, `SELECT count(*) FROM products WHERE stock_frozen`))
	mustSales(t, conn.GetContext(ctx, &setting, `SELECT freeze_new_products FROM settings WHERE id = 1`))
	if n < 1 || left != 0 || setting {
		t.Fatalf("unfroze %d, left %d, setting %v", n, left, setting)
	}
	eq(t, "a untouched", onHand(t, conn, a), 5)
}
