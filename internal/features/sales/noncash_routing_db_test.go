package sales

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// A card/online tender with a chosen account fires the non-cash money hook once,
// with the right method/amount/account; cash (drawer), credit (a debt), and a
// card line with no account picked (the record-only fallback) never fire it. This
// pins the routing guard in Create — the money-critical decision of what lands in
// an account vs. what does not.
func TestNonCashLegFiresForCardOnlineWithAccountOnly(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	var categoryID, unitID, productID, cashierID, sinceSaleID int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &sinceSaleID, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST noncash routing', $1, $2, 10, 25) RETURNING id`, categoryID, unitID))
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, sinceSaleID)                 //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, productID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, productID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, productID)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)                //nolint:errcheck
	}()

	var calls []NonCashPayment
	svc := NewService(conn).WithNonCashLeg(func(ctx context.Context, tx *sqlx.Tx, ev NonCashPayment) error {
		calls = append(calls, ev)
		return nil
	})

	ring := func(p PaymentInput) {
		t.Helper()
		_, err := svc.Create(ctx, CreateInput{
			SaleType: "retail",
			Items:    []ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}},
			Payments: []PaymentInput{p},
		}, cashierID)
		mustSales(t, err)
	}

	// Card with an account → one hook call, method/amount/account carried through.
	calls = nil
	ring(PaymentInput{Method: "card", Amount: "25", AccountID: 42})
	if len(calls) != 1 {
		t.Fatalf("card+account: hook calls = %d, want 1", len(calls))
	}
	if calls[0].Method != "card" || calls[0].AccountID != 42 || !calls[0].Amount.Equal(decimal.RequireFromString("25")) {
		t.Errorf("card+account: got %+v, want method=card account=42 amount=25", calls[0])
	}
	if calls[0].SaleID == 0 {
		t.Error("card+account: hook got SaleID 0, want the new sale id")
	}

	// Online with an account → one hook call.
	calls = nil
	ring(PaymentInput{Method: "online", Amount: "25", AccountID: 7})
	if len(calls) != 1 || calls[0].Method != "online" {
		t.Errorf("online+account: calls = %d (want 1), method = %q (want online)", len(calls), lastMethod(calls))
	}

	// Cash → drawer, never the account hook.
	calls = nil
	ring(PaymentInput{Method: "cash", Amount: "25"})
	if len(calls) != 0 {
		t.Errorf("cash: hook calls = %d, want 0", len(calls))
	}

	// Card/online with NO account picked → refused (the money would reach no
	// account), hook not fired.
	calls = nil
	for _, m := range []string{"card", "online"} {
		if _, err := svc.Create(ctx, CreateInput{
			SaleType: "retail",
			Items:    []ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}},
			Payments: []PaymentInput{{Method: m, Amount: "25"}},
		}, cashierID); err == nil {
			t.Errorf("%s without account: want refusal, got a sale", m)
		}
	}
	if len(calls) != 0 {
		t.Errorf("card without account: hook calls = %d, want 0", len(calls))
	}
}

func lastMethod(cs []NonCashPayment) string {
	if len(cs) == 0 {
		return ""
	}
	return cs[len(cs)-1].Method
}

// A plugin tender (a method core doesn't settle itself) is
// dispatched to the plugin-tender hook inside the sale tx, carrying its Meta;
// core methods (cash/card/online/credit) never reach it. Pins coreTender + the
// in-tx dispatch that makes a plugin tender atomic with the sale.
func TestPluginTenderLegFiresForNonCoreMethodOnly(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// A non-core tender value only exists once a plugin migration adds it to the
	// payment_method enum. Core-only databases have none, so there is nothing to dispatch.
	var method string
	mustSales(t, conn.GetContext(ctx, &method, `SELECT COALESCE(MIN(e.enumlabel::text), '') FROM pg_enum e
		JOIN pg_type ty ON ty.oid = e.enumtypid WHERE ty.typname = 'payment_method'
		AND e.enumlabel NOT IN ('cash','card','online','credit')`))
	if method == "" {
		t.Skip("no plugin tender in payment_method")
	}

	var categoryID, unitID, productID, cashierID, sinceSaleID int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &sinceSaleID, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST plugin tender', $1, $2, 10, 25) RETURNING id`, categoryID, unitID))
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, sinceSaleID)                 //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, productID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, productID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, productID)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)                //nolint:errcheck
	}()

	var calls []PluginTender
	svc := NewService(conn).WithPluginTenders(func(m string) bool { return m == method }, func(ctx context.Context, tx *sqlx.Tx, ev PluginTender) error {
		calls = append(calls, ev)
		return nil
	})
	ring := func(p PaymentInput) {
		t.Helper()
		_, err := svc.Create(ctx, CreateInput{
			SaleType: "retail",
			Items:    []ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}},
			Payments: []PaymentInput{p},
		}, cashierID)
		mustSales(t, err)
	}

	// A non-core method → dispatched once, method + meta carried.
	calls = nil
	ring(PaymentInput{Method: method, Amount: "25", Meta: map[string]string{"place": "9"}})
	if len(calls) != 1 || calls[0].Method != method || calls[0].Meta["place"] != "9" {
		t.Fatalf("%s: calls=%d method/meta wrong: %+v", method, len(calls), calls)
	}

	// Core methods never reach the plugin hook.
	for _, m := range []struct {
		method  string
		account int64
	}{{"cash", 0}, {"card", 42}, {"online", 7}} {
		calls = nil
		ring(PaymentInput{Method: m.method, Amount: "25", AccountID: m.account})
		if len(calls) != 0 {
			t.Errorf("%s: plugin hook fired %d times, want 0", m.method, len(calls))
		}
	}
}
