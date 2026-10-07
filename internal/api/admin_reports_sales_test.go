package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

// Sales report: the KPIs cover every sale in range (not the 50-row page), the
// previous equal period is compared, the hour window is shop time, and Peak
// hours buckets by the shop's weekday and hour.
func TestSalesReportAndPeakHours(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var cashier int64
	if err := te.db.Get(&cashier, `SELECT id FROM users ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM sales WHERE receipt_no LIKE 'T-RS-%'`) }) //nolint:errcheck
	loc := datetime.Location
	add := func(n int, at time.Time, total string) {
		for i := 0; i < n; i++ {
			te.db.MustExec(`INSERT INTO sales (receipt_no, subtotal, total, paid_amount, status, cashier_id, created_at)
				VALUES ($1, $2, $2, $2, 'completed', $3, $4)`, fmt.Sprintf("T-RS-%d-%d-%d", at.Day(), at.Hour(), i), total, cashier, at)
		}
	}
	// April 2099: 40 at 09:15 on Thu 2 Apr, 15 at 20:40 on Sat 4 Apr. March: 5.
	add(40, time.Date(2099, 4, 2, 9, 15, 0, 0, loc), "100")
	add(15, time.Date(2099, 4, 4, 20, 40, 0, 0, loc), "200")
	add(5, time.Date(2099, 3, 20, 12, 0, 0, 0, loc), "100")

	q := "from=2099-04-01&to=2099-04-30"
	r := decode[SalesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/sales?"+q, nil, admin))
	if r.Sum.Count != 55 || r.Sum.Revenue != "7000.00" || r.Sum.AvgBasket != "127.27" || r.Prev.Count != 5 || r.Grain != "day" {
		t.Fatalf("report %+v", r)
	}
	if len(r.Trend) != 2 || r.Trend[0].Day != "2099-04-02" || r.Trend[1].Revenue != "3000.00" {
		t.Fatalf("trend %+v", r.Trend)
	}
	// Every row is reachable through the list's pages, and the export has them all.
	seen := 0
	for off := 0; ; off += counterPage {
		rows := decode[[]AdminSaleRow](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/sales?%s&offset=%d", q, off), nil, admin))
		seen += len(rows)
		if len(rows) < counterPage {
			break
		}
	}
	exp := te.do(http.MethodGet, "/api/v1/admin/sales/export?format=csv&"+q, nil, admin)
	recs, _ := csv.NewReader(strings.NewReader(exp.Body.String())).ReadAll()
	if seen != 55 || len(recs)-1 != 55 || !strings.HasPrefix(recs[1][1], "2099-04-04 20:40") {
		t.Fatalf("pages %d, export %d, first %v", seen, len(recs)-1, recs[1])
	}
	// The hour window is local time and inclusive.
	if h := decode[SalesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/sales?fromHour=8&toHour=9&"+q, nil, admin)); h.Sum.Count != 40 {
		t.Fatalf("08–09 window: %d", h.Sum.Count)
	}
	// Empty range: zeros, no error.
	if e := decode[SalesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/sales?from=2098-01-01&to=2098-01-02", nil, admin)); e.Sum.Count != 0 || e.Sum.Margin != "0.0" || len(e.Trend) != 0 {
		t.Fatalf("empty %+v", e)
	}

	p := decode[PeakHoursReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/peak-hours?"+q, nil, admin))
	cell := func(dow, hour int) int {
		for _, c := range p.Cells {
			if c.Dow == dow && c.Hour == hour {
				return c.Count
			}
		}
		return 0
	}
	if p.Count != 55 || cell(4, 9) != 40 || cell(6, 20) != 15 || p.Revenue != "7000.00" {
		t.Fatalf("peak %+v", p)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/reports/peak-hours?format=csv&"+q, nil, admin); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Thursday,09,40,4000.00") {
		t.Fatalf("peak csv %d %s", rec.Code, rec.Body.String())
	}
}
