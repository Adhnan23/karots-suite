package customers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// TestRefundAdvance proves an advance can be handed back up to (not beyond) its
// size, moving the balance toward zero and showing on the statement as a debit.
func TestRefundAdvance(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	svc := NewService(conn)

	phone := fmt.Sprintf("%010d", time.Now().UnixNano()%10000000000)
	cust, _, err := svc.Create(ctx, CreateInput{Name: "refund-test-" + phone, Phone: &phone})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, cust.ID)
	if err := NewRepository(conn).AddBalance(ctx, cust.ID, decimal.RequireFromString("-500")); err != nil {
		t.Fatal(err)
	}
	refund := func(amt string) error {
		return appdb.WithTx(ctx, conn, func(tx *sqlx.Tx) error {
			_, err := svc.RefundAdvanceTx(ctx, tx, cust.ID, amt, 0)
			return err
		})
	}
	if err := refund("600"); err == nil {
		t.Fatal("refund beyond the advance must be rejected")
	}
	if err := refund("300"); err != nil {
		t.Fatal(err)
	}
	c, _ := svc.Get(ctx, cust.ID)
	if got := c.OutstandingBalance.StringFixed(2); got != "-200.00" {
		t.Errorf("balance after refund = %s, want -200.00", got)
	}
	st, err := svc.Statement(ctx, cust.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := st.Entries[len(st.Entries)-1]
	if last.Kind != "Advance refunded" || !last.Debit.Equal(decimal.RequireFromString("300")) {
		t.Errorf("statement last entry = %+v", last)
	}
}
