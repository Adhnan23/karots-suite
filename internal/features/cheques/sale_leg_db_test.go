package cheques

import (
	"context"
	"os"
	"testing"

	"karots-suite/internal/db"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

// A till sale paid 300 by cheque + 200 cash: the cheque is recorded against the
// sale and its payment line, its money sits in Cheques in hand, and the sale's
// payments show the cheque. Sales commit their own tx, so this test commits
// real rows and removes them after.
func TestSaleLegBooksCheque(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var categoryID, unitID, cashierID, since, productID, customerID int64
	must(conn.GetContext(ctx, &categoryID, `SELECT id FROM categories LIMIT 1`))
	must(conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(conn.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	must(conn.GetContext(ctx, &since, `SELECT COALESCE(MAX(id),0) FROM sales`))
	must(conn.GetContext(ctx, &productID, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST cheque sale leg', $1, $2, 100, 500) RETURNING id`, categoryID, unitID))
	must(conn.GetContext(ctx, &customerID, `INSERT INTO customers (name) VALUES ('TEST cheque sale payer') RETURNING id`))
	inHand, err := lockers.ChequesInHandTx(ctx, conn)
	must(err)
	defer func() {
		conn.ExecContext(ctx, `DELETE FROM locker_ledger WHERE ref_kind = 'sale' AND ref_id IN (SELECT id FROM sales WHERE id > $1)`, since)  //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM money_receipts WHERE ref_kind = 'sale' AND ref_id IN (SELECT id FROM sales WHERE id > $1)`, since) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM cheques WHERE customer_id = $1`, customerID)                                                       //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM sales WHERE id > $1`, since)                                                                       //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_movements WHERE product_id = $1`, productID)                                                 //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock_batches WHERE product_id = $1`, productID)                                                   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM stock WHERE product_id = $1`, productID)                                                           //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, productID)                                                                //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, customerID)                                                              //nolint:errcheck
	}()
	inHandBal := func() (d decimal.Decimal) {
		must(conn.GetContext(ctx, &d, `SELECT COALESCE(SUM(balance_delta),0) FROM locker_ledger WHERE locker_id = $1`, inHand))
		return d
	}
	before := inHandBal()

	cf := cashflow.NewService(conn, sales.NewService(conn))
	svc := sales.NewService(conn).WithChequeLeg(SaleLeg(cf))
	d, err := svc.Create(ctx, sales.CreateInput{SaleType: "retail", CustomerID: &customerID,
		Items: []sales.ItemInput{{ProductID: productID, Quantity: "1", AllowOversell: true}},
		Payments: []sales.PaymentInput{
			{Method: sales.MethodCheque, Amount: "300", Cheque: &sales.ChequeInput{Bank: "Bank of Ceylon", Number: "778899", Date: "2026-10-29"}},
			{Method: "cash", Amount: "200"},
		}}, cashierID)
	must(err)

	if got := inHandBal().Sub(before); !got.Equal(decimal.NewFromInt(300)) {
		t.Fatalf("Cheques in hand moved %s, want 300", got)
	}
	var c Cheque
	must(conn.GetContext(ctx, &c, `SELECT * FROM cheques WHERE customer_id = $1`, customerID))
	if c.Direction != "in" || c.Status != "pending" || c.SaleID == nil || *c.SaleID != d.Sale.ID || c.PaymentID == nil ||
		c.Number != "778899" || c.Bank != "Bank of Ceylon" || !c.Amount.Equal(decimal.NewFromInt(300)) {
		t.Fatalf("cheque row %+v", c)
	}
	var found bool
	for _, p := range d.Payments {
		if p.Method == sales.MethodCheque {
			found = p.ChequeNumber != nil && *p.ChequeNumber == "778899" && p.ChequeBank != nil && *p.ChequeBank == "Bank of Ceylon" &&
				p.ChequeStatus != nil && *p.ChequeStatus == "pending" && p.ChequeDate != nil
		}
	}
	if !found {
		t.Fatalf("sale payments don't show the cheque: %+v", d.Payments)
	}
}
