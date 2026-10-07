package reports

import (
	"context"
	"testing"
	"time"

	"karots-suite/internal/datetime"

	"github.com/shopspring/decimal"
)

// The trend sits next to the Revenue and Gross profit tiles, so its days must
// add up to them: the bill discount, pass-through face value and returns (at
// what was paid) all count the same way as in Compute. Days are the shop's
// days — a 00:30 sale belongs to that local date, not the UTC one before it.
func TestTrendAgreesWithPLInShopTime(t *testing.T) {
	conn := reportsTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	mustR(t, err)
	defer tx.Rollback() //nolint:errcheck

	var catID, unitID, cashierID, normalID, ptID int64
	mustR(t, tx.GetContext(ctx, &catID, `SELECT id FROM categories LIMIT 1`))
	mustR(t, tx.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	mustR(t, tx.GetContext(ctx, &cashierID, `SELECT id FROM users LIMIT 1`))
	mustR(t, tx.GetContext(ctx, &normalID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('T trend normal', $1, $2, 60, 100) RETURNING id`, catID, unitID))
	mustR(t, tx.GetContext(ctx, &ptID, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service, pass_through)
		VALUES ('T trend airtime', $1, $2, 0, 500, true, true) RETURNING id`, catID, unitID))

	loc := datetime.Location
	day1 := time.Date(2099, 3, 10, 0, 0, 0, 0, loc)
	// A: 2 × 100 + 500 airtime, 20 off the bill, one of the two returned.
	// B: 00:30 local on the next day — still the previous day in UTC.
	var a, b int64
	mustR(t, tx.GetContext(ctx, &a, `
		INSERT INTO sales (receipt_no, subtotal, discount, total, paid_amount, status, cashier_id, created_at)
		VALUES ('T-TREND-A', 700, 20, 680, 680, 'completed', $1, $2) RETURNING id`, cashierID, day1.Add(10*time.Hour)))
	mustR(t, tx.GetContext(ctx, &b, `
		INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, created_at)
		VALUES ('T-TREND-B', 100, 100, 100, 'completed', $1, $2) RETURNING id`, cashierID, day1.Add(24*time.Hour+30*time.Minute)))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal, cost_price, returned_qty)
		VALUES ($1,$3,2,100,200,60,1), ($1,$4,1,500,500,0,0), ($2,$3,1,100,100,60,0)`, a, b, normalID, ptID)
	mustR(t, err)

	svc := &Service{db: tx}
	from, to := day1, day1.AddDate(0, 0, 2)
	pl, err := svc.Compute(ctx, from, to)
	mustR(t, err)
	// The refund for one of A's two lines is 100 × 680/700, not the 100 sticker.
	if got := pl.Returns.Round(2); !got.Equal(dec("97.14")) {
		t.Fatalf("Returns = %s, want 97.14 (what was paid)", got)
	}

	rows, err := svc.SalesByPeriod(ctx, from, to, "day")
	mustR(t, err)
	if len(rows) != 2 || rows[0].Day.Format("2006-01-02") != "2099-03-10" || rows[1].Day.Format("2006-01-02") != "2099-03-11" {
		t.Fatalf("buckets = %+v, want 2099-03-10 and 2099-03-11 (shop time)", rows)
	}
	var rev, profit decimal.Decimal
	for _, r := range rows {
		rev, profit = rev.Add(r.Revenue), profit.Add(r.Profit)
	}
	if !rev.Round(2).Equal(pl.Revenue.Round(2)) || !profit.Round(2).Equal(pl.GrossProfit.Round(2)) {
		t.Fatalf("trend revenue/profit = %s/%s, P&L = %s/%s", rev, profit, pl.Revenue, pl.GrossProfit)
	}
}
