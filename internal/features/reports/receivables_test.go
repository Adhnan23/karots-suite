package reports

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// A customer holding an advance (negative balance) is money the shop OWES, not
// a smaller debt from everyone else. Summing balances raw netted the advance
// against other customers' dues and understated what the shop is owed.
func TestReceivablesExcludeCustomerAdvances(t *testing.T) {
	conn := reportsTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	// Repeatable read: other packages' tests commit customers in parallel, and
	// the P&L must see only this tx's snapshot.
	tx, err := conn.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	mustR(t, err)
	defer tx.Rollback() //nolint:errcheck

	// Only the two test customers: zero everyone else inside the rolled-back tx.
	_, err = tx.ExecContext(ctx, `UPDATE customers SET outstanding_balance = 0`)
	mustR(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO customers (name, phone, outstanding_balance) VALUES
		('T owes', 'T-recv-1', 500), ('T advance', 'T-recv-2', -200)`)
	mustR(t, err)

	from := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	pl, err := (&Service{db: tx}).Compute(ctx, from, from.AddDate(0, 0, 1))
	mustR(t, err)
	if !pl.Receivables.Equal(dec("500")) {
		t.Errorf("Receivables = %s, want 500 (dues only)", pl.Receivables)
	}
	if !pl.CustomerAdvances.Equal(dec("200")) {
		t.Errorf("CustomerAdvances = %s, want 200", pl.CustomerAdvances)
	}
}
