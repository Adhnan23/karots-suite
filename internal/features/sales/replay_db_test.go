package sales

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// replayFixture makes a throwaway product (price 120, stock qty) and cleans up
// every sale made after it, FK-safe.
func replayFixture(t *testing.T, conn *sqlx.DB, stockQty string) (productID, cashierID int64) {
	t.Helper()
	ctx := context.Background()
	var categoryID, unitID, since int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &since, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST replay', $1, $2, 50, 120) RETURNING id`, categoryID, unitID))
	if stockQty != "0" {
		_, err := conn.ExecContext(ctx, `UPDATE stock SET quantity = $2 WHERE product_id = $1`, productID, stockQty)
		mustSales(t, err)
		mustSales(t, conn.GetContext(ctx, new(int), `
			INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price) VALUES ($1, $2, $2, 50) RETURNING 1`, productID, stockQty))
	}
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, since)                       //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, productID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, productID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, productID)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)                //nolint:errcheck
	})
	return productID, cashierID
}

func flagKinds(t *testing.T, svc *Service, saleID int64) []Flag {
	t.Helper()
	fs, err := svc.Flags(context.Background(), saleID)
	mustSales(t, err)
	return fs
}

func TestCreateClientRefOnce(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	svc := NewService(conn)
	in := CreateInput{SaleType: "retail", ClientRef: "TEST-r1-" + time.Now().Format("150405.000"),
		Items:    []ItemInput{{ProductID: pid, Quantity: "1"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "120"}}}
	first, err := svc.Create(ctx, in, uid)
	mustSales(t, err)
	second, err := svc.Create(ctx, in, uid)
	mustSales(t, err)
	if second.Sale.ID != first.Sale.ID || !second.Duplicate || first.Duplicate {
		t.Fatalf("ids %d/%d dup %v/%v", first.Sale.ID, second.Sale.ID, first.Duplicate, second.Duplicate)
	}
	var n int
	mustSales(t, conn.GetContext(ctx, &n, `SELECT count(*) FROM sales WHERE client_ref = $1`, in.ClientRef))
	var onHand decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &onHand, `SELECT quantity FROM stock WHERE product_id = $1`, pid))
	if n != 1 || !onHand.Equal(decimal.NewFromInt(9)) {
		t.Fatalf("rows %d, on hand %s; want 1 and 9", n, onHand)
	}
}

func TestCreateAtAndRungAt(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	at := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	d, err := NewService(conn).Create(ctx, CreateInput{SaleType: "retail", At: &at, RungAt: &at,
		Items:    []ItemInput{{ProductID: pid, Quantity: "1"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "120"}}}, uid)
	mustSales(t, err)
	if !d.Sale.CreatedAt.Equal(at) || d.Sale.RungAt == nil || !d.Sale.RungAt.Equal(at) {
		t.Fatalf("created %v rung %v, want %v", d.Sale.CreatedAt, d.Sale.RungAt, at)
	}
}

func TestReviewKeepsExpectedPrice(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{SaleType: "retail", Review: true,
		Items:    []ItemInput{{ProductID: pid, Quantity: "1", ExpectPrice: "100"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "100"}}}, uid)
	mustSales(t, err)
	if !d.Items[0].UnitPrice.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unit price %s, want 100", d.Items[0].UnitPrice)
	}
	fs := flagKinds(t, svc, d.Sale.ID)
	if len(fs) != 1 || fs[0].Kind != FlagPrice || !strings.Contains(fs[0].Note, "100") || !strings.Contains(fs[0].Note, "120") {
		t.Fatalf("flags %+v", fs)
	}
	// Without Review the catalogue price wins.
	d, err = svc.Create(ctx, CreateInput{SaleType: "retail",
		Items:    []ItemInput{{ProductID: pid, Quantity: "1", ExpectPrice: "100"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "120"}}}, uid)
	mustSales(t, err)
	if !d.Items[0].UnitPrice.Equal(decimal.NewFromInt(120)) || len(flagKinds(t, svc, d.Sale.ID)) != 0 {
		t.Fatalf("unit price %s without review", d.Items[0].UnitPrice)
	}
}

func TestReviewOversellFlagged(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "1")
	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{SaleType: "retail", Review: true,
		Items:    []ItemInput{{ProductID: pid, Quantity: "3"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "360"}}}, uid)
	mustSales(t, err)
	var onHand decimal.Decimal
	mustSales(t, conn.GetContext(ctx, &onHand, `SELECT quantity FROM stock WHERE product_id = $1`, pid))
	fs := flagKinds(t, svc, d.Sale.ID)
	if !onHand.IsZero() || len(fs) != 1 || fs[0].Kind != FlagOversold {
		t.Fatalf("on hand %s flags %+v", onHand, fs)
	}
}

func TestReviewOverLimitFlagged(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	pid, uid := replayFixture(t, conn, "10")
	var custID int64
	mustSales(t, conn.GetContext(ctx, &custID, `
		INSERT INTO customers (name, credit_limit) VALUES ('TEST replay limit', 100) RETURNING id`))
	t.Cleanup(func() { conn.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, custID) }) //nolint:errcheck
	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{SaleType: "retail", Review: true, CustomerID: &custID,
		Items:    []ItemInput{{ProductID: pid, Quantity: "3"}},
		Payments: []PaymentInput{{Method: "credit", Amount: "360"}}}, uid)
	mustSales(t, err)
	fs := flagKinds(t, svc, d.Sale.ID)
	if d.Sale.Status != "credit" || len(fs) != 1 || fs[0].Kind != FlagOverLimit {
		t.Fatalf("status %s flags %+v", d.Sale.Status, fs)
	}
}
