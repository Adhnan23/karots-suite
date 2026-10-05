package reports

import (
	"context"
	"testing"
	"time"
)

// "Received at sale" is what customers handed over and kept with the shop:
// the tender less the change given back. Summing paid_amount alone counted
// every rupee of change as money received.
func TestReceivedIsNetOfChange(t *testing.T) {
	conn := reportsTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	mustR(t, err)
	defer tx.Rollback() //nolint:errcheck

	at := time.Date(2099, 5, 1, 10, 0, 0, 0, time.UTC)
	var cashier int64
	mustR(t, tx.GetContext(ctx, &cashier, `SELECT id FROM users ORDER BY id LIMIT 1`))
	_, err = tx.ExecContext(ctx, `
		WITH s AS (INSERT INTO sales (receipt_no, total, subtotal, paid_amount, change_given, cashier_id, created_at) VALUES
			('T-RCV-1', 200, 200, 5000, 4800, $1, $2), ('T-RCV-2', 300, 300, 300, 0, $1, $2) RETURNING id, receipt_no)
		INSERT INTO payments (sale_id, method, amount)
		SELECT id, CASE receipt_no WHEN 'T-RCV-1' THEN 'cash' ELSE 'card' END::payment_method, CASE receipt_no WHEN 'T-RCV-1' THEN 5000 ELSE 300 END FROM s`, cashier, at)
	mustR(t, err)
	pl, err := (&Service{db: tx}).Compute(ctx, at.Truncate(24*time.Hour), at.Truncate(24*time.Hour).AddDate(0, 0, 1))
	mustR(t, err)
	if !pl.Received.Equal(dec("500")) {
		t.Fatalf("Received = %s, want 500 (tender less change)", pl.Received)
	}
	// The payment mix too: change leaves as cash, so cash is net of it.
	for _, m := range pl.ByPayment {
		if want := map[string]string{"cash": "200", "card": "300"}[m.Method]; !m.Total.Equal(dec(want)) {
			t.Fatalf("ByPayment %s = %s, want %s", m.Method, m.Total, want)
		}
	}
}
