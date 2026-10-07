package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

// The dashboard's today is the shop's today (a 00:30 sale counts), the chart
// periods are dense and line up with the period before, and only admin /
// manager can read it.
func TestDashboard(t *testing.T) {
	te := newTestEnv(t)
	phone := func() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }
	admin := te.login(t, phone(), "admin")
	cashier := te.login(t, phone(), "cashier")
	var who int64
	te.db.Get(&who, `SELECT id FROM users ORDER BY id LIMIT 1`) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`DELETE FROM sales WHERE receipt_no = 'T-DASH-1'`) }) //nolint:errcheck

	before := decode[Dashboard](t, te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, admin))
	now := time.Now().In(datetime.Location)
	early := time.Date(now.Year(), now.Month(), now.Day(), 0, 30, 0, 0, datetime.Location)
	te.db.MustExec(`INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, created_at) VALUES ('T-DASH-1', 250, 250, 250, 'completed', $1, $2)`, who, early)
	after := decode[Dashboard](t, te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, admin))
	if after.Today.Count != before.Today.Count+1 || len(after.Spark) != 14 || after.Spark[13].Day != now.Format("2006-01-02") {
		t.Fatalf("today %d → %d, spark %d ending %s", before.Today.Count, after.Today.Count, len(after.Spark), after.Spark[len(after.Spark)-1].Day)
	}
	ch := decode[DashboardCharts](t, te.do(http.MethodGet, "/api/v1/admin/dashboard/charts?days=7", nil, admin))
	if len(ch.Cur) != 7 || len(ch.Prev) != 7 || ch.Grain != "day" || ch.Cur[6].Day != now.Format("2006-01-02") || ch.Prev[6].Day != now.AddDate(0, 0, -7).Format("2006-01-02") {
		t.Fatalf("charts %+v", ch)
	}
	if c := te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, cashier).Code; c != http.StatusForbidden {
		t.Fatalf("cashier: %d", c)
	}
}
