package products

import (
	"context"
	"testing"
)

func TestProductReadsStockFrozen(t *testing.T) {
	conn, a, _ := barcodeFixture(t)
	ctx := context.Background()
	_, err := conn.ExecContext(ctx, `UPDATE products SET stock_frozen = true WHERE id = $1`, a)
	must(t, err)
	p, err := NewService(conn).Get(ctx, a)
	must(t, err)
	if !p.StockFrozen {
		t.Fatal("StockFrozen not read")
	}
}

func TestNewProductStartsFrozenWhenShopFrozen(t *testing.T) {
	conn, _, _ := barcodeFixture(t)
	ctx := context.Background()
	s := NewService(conn)
	var catID, unitID int64
	must(t, conn.GetContext(ctx, &catID, `SELECT id FROM categories WHERE name = 'TEST barcodes cat'`))
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	var ids []int64
	t.Cleanup(func() {
		conn.ExecContext(ctx, `UPDATE settings SET freeze_new_products = false WHERE id = 1`) //nolint:errcheck
		for _, id := range ids {
			conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id) //nolint:errcheck
		}
	})
	var user int64
	must(t, conn.GetContext(ctx, &user, `SELECT id FROM users ORDER BY id LIMIT 1`))
	for _, on := range []bool{true, false} {
		_, err := conn.ExecContext(ctx, `UPDATE settings SET freeze_new_products = $1 WHERE id = 1`, on)
		must(t, err)
		suffix := map[bool]string{true: "on", false: "off"}[on]
		p, err := s.Create(ctx, CreateInput{Name: "TEST frz create " + suffix, CategoryID: catID, UnitID: unitID})
		must(t, err)
		ids = append(ids, p.ID)
		q, err := s.CreateForIntake(ctx, IntakeInput{Name: "TEST frz intake " + suffix}, user)
		must(t, err)
		ids = append(ids, q.ID)
		if p.StockFrozen != on || q.StockFrozen != on {
			t.Fatalf("setting %v: create %v, intake %v", on, p.StockFrozen, q.StockFrozen)
		}
	}
	// A service never starts frozen: it has no stock.
	_, err := conn.ExecContext(ctx, `UPDATE settings SET freeze_new_products = true WHERE id = 1`)
	must(t, err)
	sv, err := s.Create(ctx, CreateInput{Name: "TEST frz service", CategoryID: catID, UnitID: unitID, IsService: true})
	must(t, err)
	ids = append(ids, sv.ID)
	if sv.StockFrozen {
		t.Fatal("service started frozen")
	}
}
