package sales

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

// The line hook gets only the plugin-tagged lines, with the sale id, inside the
// sale's tx — and a hook error rolls the whole sale back.
func TestLineHookSeesTaggedLinesInTheSaleTx(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	var categoryID, unitID, productID, cashierID, before, after int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('TEST line hook', $1, $2, 0, 0, true) RETURNING id`, categoryID, unitID))
	defer conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID) //nolint:errcheck

	in := CreateInput{SaleType: "retail", Items: []ItemInput{
		{ProductID: productID, Quantity: "1", PriceOverride: "10"},
		{ProductID: productID, Quantity: "2", PriceOverride: "10", Plugin: map[string]json.RawMessage{"x": json.RawMessage(`{"job":7}`)}},
	}, Payments: []PaymentInput{{Method: "cash", Amount: "30"}}}

	var got TaggedSale
	var inTx bool
	d, err := NewService(conn).WithLineHook(func(ctx context.Context, tx *sqlx.Tx, ev TaggedSale) error {
		got = ev
		// The sale row is visible inside the hook's tx (it's the same tx).
		inTx = tx.GetContext(ctx, new(int64), `SELECT id FROM sales WHERE id = $1`, ev.SaleID) == nil
		return nil
	}).Create(ctx, in, cashierID)
	mustSales(t, err)
	defer conn.ExecContext(ctx, `DELETE FROM sales WHERE id = $1`, d.Sale.ID) //nolint:errcheck
	if got.SaleID != d.Sale.ID || len(got.Lines) != 1 || got.Lines[0].ItemID != d.Items[1].ID || got.Lines[0].Amount.String() != "20" ||
		got.Lines[0].Quantity.String() != "2" || string(got.Lines[0].Plugin["x"]) != `{"job":7}` || !inTx {
		t.Fatalf("hook saw %+v (in tx %v)", got, inTx)
	}

	mustSales(t, conn.GetContext(ctx, &before, `SELECT count(*) FROM sales`))
	_, err = NewService(conn).WithLineHook(func(context.Context, *sqlx.Tx, TaggedSale) error { return errors.New("boom") }).Create(ctx, in, cashierID)
	mustSales(t, conn.GetContext(ctx, &after, `SELECT count(*) FROM sales`))
	if err == nil || after != before {
		t.Fatalf("a failing hook kept the sale: err %v, sales %d → %d", err, before, after)
	}
}

// The return hook gets the returned lines with their refund and the plugin data
// the till sent for each, inside the return's tx; its error rolls the return back.
func TestReturnHookSeesReturnedLines(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	var categoryID, unitID, productID, cashierID int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('TEST return hook', $1, $2, 0, 0, true) RETURNING id`, categoryID, unitID))
	defer conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID) //nolint:errcheck
	svc := NewService(conn)
	d, err := svc.Create(ctx, CreateInput{SaleType: "retail", Items: []ItemInput{{ProductID: productID, Quantity: "1", PriceOverride: "100"}},
		Payments: []PaymentInput{{Method: "cash", Amount: "100"}}}, cashierID)
	mustSales(t, err)
	defer conn.ExecContext(ctx, `DELETE FROM sales WHERE id = $1`, d.Sale.ID) //nolint:errcheck
	in := PartialReturnInput{Lines: []ReturnLineInput{{SaleItemID: d.Items[0].ID, Quantity: "1",
		Plugin: map[string]json.RawMessage{"x": json.RawMessage(`{"back":"all"}`)}}}}

	_, _, err = svc.WithReturnHook(func(context.Context, *sqlx.Tx, ReturnedSale) error { return errors.New("boom") }).PartialReturn(ctx, d.Sale.ID, in, cashierID)
	if err == nil {
		t.Fatal("a failing return hook kept the return")
	}
	var got ReturnedSale
	_, _, err = svc.WithReturnHook(func(_ context.Context, _ *sqlx.Tx, ev ReturnedSale) error { got = ev; return nil }).PartialReturn(ctx, d.Sale.ID, in, cashierID)
	mustSales(t, err)
	if got.SaleID != d.Sale.ID || got.ReturnID == 0 || len(got.Lines) != 1 || got.Lines[0].SaleItemID != d.Items[0].ID ||
		got.Lines[0].Refund.String() != "100" || string(got.Lines[0].Plugin["x"]) != `{"back":"all"}` {
		t.Fatalf("return hook saw %+v", got)
	}
}
