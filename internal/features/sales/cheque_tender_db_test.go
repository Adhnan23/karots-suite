package sales

import (
	"context"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// chequeFixture is a product priced 500 and a named customer, cleaned up after.
func chequeFixture(t *testing.T) (conn *sqlx.DB, productID, customerID, cashierID int64) {
	t.Helper()
	conn = salesTestDB(t)
	ctx := context.Background()
	var categoryID, unitID, since int64
	mustSales(t, conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustSales(t, conn.GetContext(ctx, &since, `SELECT COALESCE(MAX(id),0) FROM sales`))
	mustSales(t, conn.GetContext(ctx, &productID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST cheque tender', $1, $2, 100, 500) RETURNING id`, categoryID, unitID))
	mustSales(t, conn.GetContext(ctx, &customerID, `INSERT INTO customers (name) VALUES ('TEST cheque payer') RETURNING id`))
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, since)                       //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, productID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, productID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, productID)           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)                //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, customerID)              //nolint:errcheck
		conn.Close()
	})
	return conn, productID, customerID, cashierID
}

func chequeSale(productID int64, customer *int64, ps ...PaymentInput) CreateInput {
	return CreateInput{SaleType: "retail", CustomerID: customer,
		Items: []ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}}, Payments: ps}
}

func cheque(amount string) PaymentInput {
	return PaymentInput{Method: MethodCheque, Amount: amount, Cheque: &ChequeInput{Bank: "Bank of Ceylon", Number: "123456", Date: "2026-10-29"}}
}

func wantValidation(t *testing.T, err error, what string) {
	t.Helper()
	if ae, ok := apperr.As(err); !ok || ae.Code != "VALIDATION_ERROR" {
		t.Fatalf("%s: err = %v, want a validation error", what, err)
	}
}

func TestChequeTenderNeedsCustomer(t *testing.T) {
	conn, productID, _, cashierID := chequeFixture(t)
	_, err := NewService(conn).Create(context.Background(), chequeSale(productID, nil, cheque("500")), cashierID)
	wantValidation(t, err, "walk-in cheque")
}

func TestChequeTenderNeedsNumberAndDate(t *testing.T) {
	conn, productID, customerID, cashierID := chequeFixture(t)
	p := cheque("500")
	p.Cheque.Number = ""
	_, err := NewService(conn).Create(context.Background(), chequeSale(productID, &customerID, p), cashierID)
	wantValidation(t, err, "no number")
	p = cheque("500")
	p.Cheque = nil
	_, err = NewService(conn).Create(context.Background(), chequeSale(productID, &customerID, p), cashierID)
	wantValidation(t, err, "no cheque details")
}

func TestChequeTenderOverDue(t *testing.T) {
	conn, productID, customerID, cashierID := chequeFixture(t)
	_, err := NewService(conn).Create(context.Background(), chequeSale(productID, &customerID, cheque("600")), cashierID)
	wantValidation(t, err, "cheque over the bill")
}

func TestChequeTenderBooks(t *testing.T) {
	conn, productID, customerID, cashierID := chequeFixture(t)
	var got []ChequeTaken
	svc := NewService(conn).WithChequeLeg(func(_ context.Context, _ *sqlx.Tx, ev ChequeTaken) error {
		got = append(got, ev)
		return nil
	})
	d, err := svc.Create(context.Background(), chequeSale(productID, &customerID, cheque("300"), PaymentInput{Method: "cash", Amount: "200"}), cashierID)
	mustSales(t, err)
	if d.Sale.Status != "completed" || !d.Sale.PaidAmount.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("sale status %q paid %s, want completed 500", d.Sale.Status, d.Sale.PaidAmount)
	}
	if len(got) != 1 {
		t.Fatalf("cheque leg calls = %d, want 1", len(got))
	}
	ev := got[0]
	if ev.SaleID != d.Sale.ID || ev.CustomerID != customerID || ev.PaymentID == 0 || ev.Party != "TEST cheque payer" ||
		!ev.Amount.Equal(decimal.NewFromInt(300)) || ev.Cheque.Number != "123456" || ev.ActorID != cashierID {
		t.Fatalf("cheque leg got %+v", ev)
	}
	var payID int64
	mustSales(t, conn.Get(&payID, `SELECT id FROM payments WHERE sale_id = $1 AND method = 'cheque'`, d.Sale.ID))
	if payID != ev.PaymentID {
		t.Fatalf("leg payment id %d, want the cheque payment row %d", ev.PaymentID, payID)
	}
}

func TestChequeNotPluginTender(t *testing.T) {
	conn, productID, customerID, cashierID := chequeFixture(t)
	asked := false
	svc := NewService(conn).WithPluginTenders(func(m string) bool {
		if m == MethodCheque {
			asked = true
		}
		return false
	}, func(context.Context, *sqlx.Tx, PluginTender) error {
		t.Fatal("cheque dispatched to the plugin tender leg")
		return nil
	})
	_, err := svc.Create(context.Background(), chequeSale(productID, &customerID, cheque("500")), cashierID)
	mustSales(t, err)
	if asked {
		t.Fatal("plugin tender known-func was asked about cheque")
	}
}

func TestChequeIsCoreTender(t *testing.T) {
	if got := coreTender(MethodCheque); !got {
		t.Fatal("cheque must be a core tender")
	}
}
