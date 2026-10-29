package api

import (
	"fmt"
	"net/http"
	"time"

	"karots-suite/internal/features/reports"
	"karots-suite/internal/features/sales"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsSales(g *echo.Group) {
	g.GET("/reports/sales", h.ReportSales)
	g.GET("/reports/peak-hours", h.ReportPeakHours)
}

func salesReportSum(s *sales.ListSummary) SalesReportSum {
	return SalesReportSum{Count: s.Count, Revenue: dec2(s.NetRev), Total: dec2(s.Net), Discount: dec2(s.Discount), COGS: dec2(s.COGS),
		Profit: dec2(s.Profit), Margin: s.Margin().StringFixed(1), AvgBasket: dec2(s.AvgBasket())}
}

// ReportSales is the Sales report's head: the totals
// over the whole range, the same filters over the period before, and the trend.
// Its receipts, drill-in and export are the admin sales list's
// (/admin/sales, /admin/sales/:id, /admin/sales/export) with the same filters.
func (h *handlers) ReportSales(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	f, err := salesFilter(c)
	if err != nil {
		return err
	}
	f.From, f.To = &from, &to
	sum, err := h.d.Sales.Summarize(ctx, f)
	if err != nil {
		return err
	}
	pf, pt := prevPeriod(from, to)
	f.From, f.To = &pf, &pt
	prev, err := h.d.Sales.Summarize(ctx, f)
	if err != nil {
		return err
	}
	grain := financeGrain(int(to.Sub(from).Hours() / 24))
	trend, err := reports.NewService(h.d.DB).SalesByPeriod(ctx, from, to, grain)
	if err != nil {
		return err
	}
	out := SalesReport{From: fs, To: ts, Grain: grain, Sum: salesReportSum(sum), Prev: salesReportSum(prev), Trend: make([]FinancePoint, 0, len(trend))}
	for _, p := range trend {
		out.Trend = append(out.Trend, FinancePoint{Day: p.Day.Format("2006-01-02"), Revenue: dec2(p.Revenue), Profit: dec2(p.Profit)})
	}
	return c.JSON(http.StatusOK, out)
}

// ReportPeakHours is sales by shop-time weekday × hour,
// the staffing view.
func (h *handlers) ReportPeakHours(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	buckets, err := reports.NewService(h.d.DB).PeakHours(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	if wantsSheet(c) {
		rows := make([][]string, 0, len(buckets))
		for _, b := range buckets {
			rows = append(rows, []string{time.Weekday(b.DOW).String(), fmt.Sprintf("%02d", b.Hour), fmt.Sprint(b.Count), dec2(b.Revenue)})
		}
		return writeSheet(c, "peak_hours_"+fs+"_"+ts, []string{"Day", "Hour", "Sales", "Revenue"}, rows)
	}
	out := PeakHoursReport{From: fs, To: ts, Cells: make([]PeakCell, 0, len(buckets))}
	rev := decimal.Zero
	for _, b := range buckets {
		out.Cells = append(out.Cells, PeakCell{Dow: b.DOW, Hour: b.Hour, Count: b.Count, Revenue: dec2(b.Revenue)})
		out.Count += b.Count
		rev = rev.Add(b.Revenue)
	}
	out.Revenue = dec2(rev)
	return c.JSON(http.StatusOK, out)
}
