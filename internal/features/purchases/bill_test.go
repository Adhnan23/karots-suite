package purchases

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// A bill with no lines (a supplier's float, a service) is a received purchase the
// shop owes in full: it lands on the supplier balance and the open-invoice queue
// like any delivery, and touches no stock.
func TestCreateBillOwesTheSupplier(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	must(t, err)
	defer tx.Rollback() //nolint:errcheck

	var supplierID int64
	must(t, tx.GetContext(ctx, &supplierID, `INSERT INTO suppliers (name) VALUES ('TEST bill supplier') RETURNING id`))

	if _, err := CreateBillTx(ctx, tx, BillInput{SupplierID: supplierID, Amount: decimal.Zero}, 0); err == nil {
		t.Fatal("a zero bill was accepted")
	}
	d, err := CreateBillTx(ctx, tx, BillInput{SupplierID: supplierID, Amount: decimal.NewFromInt(5000), InvoiceNo: "F-1", Notes: "float"}, 0)
	must(t, err)
	if d.Purchase.Status != "received" || !d.Purchase.Total.Equal(decimal.NewFromInt(5000)) || len(d.Items) != 0 {
		t.Fatalf("bill = %s %s with %d lines, want received 5000 with none", d.Purchase.Status, d.Purchase.Total, len(d.Items))
	}
	var owed decimal.Decimal
	must(t, tx.GetContext(ctx, &owed, `SELECT outstanding_balance FROM suppliers WHERE id = $1`, supplierID))
	if !owed.Equal(decimal.NewFromInt(5000)) {
		t.Errorf("supplier owed %s, want 5000", owed)
	}
	open, err := NewRepository(tx).OpenBySupplier(ctx, supplierID)
	must(t, err)
	if len(open) != 1 || !open[0].Balance().Equal(decimal.NewFromInt(5000)) {
		t.Errorf("open invoices = %d, want the one bill owing 5000", len(open))
	}
}
