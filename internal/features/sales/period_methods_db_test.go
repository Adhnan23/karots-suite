package sales

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// The day-end Z report's per-method lines show what each tender left with the
// shop: change always leaves as cash, so cash is net of it (a Rs 200 sale paid
// with Rs 5,000 is Rs 200 cash, not Rs 5,000).
func TestPeriodSummaryCashIsNetOfChange(t *testing.T) {
	conn := salesTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	at := time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC)
	var cashier, saleID int64
	mustSales(t, conn.Get(&cashier, `SELECT id FROM users ORDER BY id LIMIT 1`))
	mustSales(t, conn.Get(&saleID, `INSERT INTO sales (receipt_no, total, subtotal, paid_amount, change_given, cashier_id, created_at)
		VALUES ('T-PSUM-1', 200, 200, 5000, 4800, $1, $2) RETURNING id`, cashier, at))
	defer func() { // a defer, not t.Cleanup: Cleanup would run after conn.Close
		conn.Exec(`DELETE FROM payments WHERE sale_id = $1`, saleID) //nolint:errcheck
		conn.Exec(`DELETE FROM sales WHERE id = $1`, saleID)         //nolint:errcheck
	}()
	_, err := conn.Exec(`INSERT INTO payments (sale_id, method, amount) VALUES ($1, 'cash', 5000)`, saleID)
	mustSales(t, err)
	sum, err := NewService(conn).PeriodSummary(ctx, cashier, at.Add(-time.Hour), at.Add(time.Hour))
	mustSales(t, err)
	if len(sum.ByMethod) != 1 || sum.ByMethod[0].Method != "cash" || !sum.ByMethod[0].Amount.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("ByMethod = %+v, want cash 200", sum.ByMethod)
	}
}
