package sales

import (
	"context"
	"errors"
	"testing"

	"karots-suite/internal/apperr"
)

// A payment method that is neither core's nor any plugin's is refused as a
// validation error before anything is written — never recorded as money that
// lands nowhere (status.md "money must always land somewhere real").
func TestCreateRefusesUnknownTender(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	var categoryID, unitID, productID, cashierID, before, after int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST unknown tender', $1, $2, 10, 25) RETURNING id`, categoryID, unitID))
	defer conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID) //nolint:errcheck
	mustSales(t, conn.GetContext(ctx, &before, `SELECT count(*) FROM sales`))

	for _, svc := range []*Service{
		NewService(conn), // no plugin tenders wired at all
		NewService(conn).WithPluginTenders(func(string) bool { return false }, nil),
	} {
		_, err := svc.Create(ctx, CreateInput{
			SaleType: "retail",
			Items:    []ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}},
			Payments: []PaymentInput{{Method: "mystery", Amount: "25"}},
		}, cashierID)
		var ae *apperr.AppError
		if !errors.As(err, &ae) || ae.Code != apperr.Validation("").Code {
			t.Fatalf("want a validation error, got %v", err)
		}
	}
	mustSales(t, conn.GetContext(ctx, &after, `SELECT count(*) FROM sales`))
	if after != before {
		t.Fatalf("sales rows %d → %d: an unknown tender wrote a sale", before, after)
	}
}
