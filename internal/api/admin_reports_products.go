package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsProducts(g *echo.Group) {
	g.GET("/reports/top-products", h.ReportTopProducts)
	g.GET("/reports/product-sales", h.ReportProductSales)
	g.GET("/reports/sales-by-cashier", h.ReportCashiers)
}

// pct1 is part / whole as a percent with 1 dp ("0.0" when whole isn't positive).
func pct1(part, whole decimal.Decimal) string {
	if !whole.IsPositive() {
		return "0.0"
	}
	return part.Div(whole).Mul(decimal.NewFromInt(100)).StringFixed(1)
}

// ReportTopProducts ranks the best sellers by
// ?order=revenue|qty|profit, ?limit= rows (default 50, at most 200), with the
// totals of every product sold for the shares and the Pareto line.
func (h *handlers) ReportTopProducts(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	order := c.QueryParam("order")
	if order != "qty" && order != "profit" {
		order = "revenue"
	}
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	rep := reports.NewService(h.d.DB)
	rows, err := rep.TopProducts(ctx, from, to, order, limit)
	if err != nil {
		return err
	}
	if wantsSheet(c) {
		out := make([][]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, []string{r.ProductName, r.Qty.String(), dec2(r.Revenue), dec2(r.Profit), pct1(r.Profit, r.Revenue)})
		}
		return writeSheet(c, "top_products_"+fs+"_"+ts, []string{"Product", "Qty sold", "Revenue", "Profit", "Margin %"}, out)
	}
	tot, err := rep.ProductTotals(ctx, from, to)
	if err != nil {
		return err
	}
	out := TopProductsReport{From: fs, To: ts, Order: order, Products: tot.Products, Qty: tot.Qty.String(), Revenue: dec2(tot.Revenue),
		Profit: dec2(tot.Profit), Margin: pct1(tot.Profit, tot.Revenue), Rows: make([]TopProductRow, 0, len(rows))}
	for _, r := range rows {
		out.Rows = append(out.Rows, TopProductRow{ProductID: r.ProductID, Name: r.ProductName, Qty: r.Qty.String(), Revenue: dec2(r.Revenue),
			Profit: dec2(r.Profit), Margin: pct1(r.Profit, r.Revenue)})
	}
	return c.JSON(http.StatusOK, out)
}

// periodStart is the start of t's day / week (Monday, as date_trunc) / month.
func periodStart(t time.Time, grain string) time.Time {
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	switch grain {
	case "month":
		return d.AddDate(0, 0, 1-d.Day())
	case "week":
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	}
	return d
}

// ReportProductSales is one product's units and revenue by ?grain= (day |
// week | month; default by range length), beside the same periods a year
// earlier.
func (h *handlers) ReportProductSales(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	grain := c.QueryParam("grain")
	if grain != "day" && grain != "week" && grain != "month" {
		grain = financeGrain(int(to.Sub(from).Hours() / 24))
	}
	out := ProductSalesReport{From: fs, To: ts, Grain: grain, Qty: "0", Revenue: "0.00", LastYearQty: "0", Rows: []ProductPeriod{}}
	pid, _ := strconv.ParseInt(c.QueryParam("product"), 10, 64)
	if pid <= 0 {
		return c.JSON(http.StatusOK, out)
	}
	p, err := h.t.products.Get(ctx, pid)
	if err != nil {
		return err
	}
	rep := reports.NewService(h.d.DB)
	now, err := rep.ProductSalesByPeriod(ctx, pid, from, to, grain)
	if err != nil {
		return err
	}
	ly, err := rep.ProductSalesByPeriod(ctx, pid, from.AddDate(-1, 0, 0), to.AddDate(-1, 0, 0), grain)
	if err != nil {
		return err
	}
	type pair struct{ cur, last reports.ProductPeriodRow }
	byDay := map[time.Time]*pair{}
	at := func(t time.Time) *pair {
		k := periodStart(t, grain)
		if byDay[k] == nil {
			byDay[k] = &pair{}
		}
		return byDay[k]
	}
	var qty, rev, lyQty decimal.Decimal
	for _, r := range now {
		at(r.Day).cur = r
		qty, rev = qty.Add(r.Qty), rev.Add(r.Revenue)
	}
	for _, r := range ly {
		x := at(r.Day.AddDate(1, 0, 0))
		x.last.Qty, x.last.Revenue = x.last.Qty.Add(r.Qty), x.last.Revenue.Add(r.Revenue)
		lyQty = lyQty.Add(r.Qty)
	}
	days := make([]time.Time, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	for _, d := range days {
		x := byDay[d]
		out.Rows = append(out.Rows, ProductPeriod{Day: d.Format("2006-01-02"), Qty: x.cur.Qty.String(), Revenue: dec2(x.cur.Revenue),
			LastYearQty: x.last.Qty.String(), LastYearRevenue: dec2(x.last.Revenue)})
	}
	out.ProductID, out.Name, out.Qty, out.Revenue, out.LastYearQty = pid, p.Name, qty.String(), dec2(rev), lyQty.String()
	return c.JSON(http.StatusOK, out)
}

// ReportCashiers is takings per cashier, biggest
// first.
func (h *handlers) ReportCashiers(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := reports.NewService(h.d.DB).SalesByCashier(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	avg := func(net decimal.Decimal, n int) string {
		if n == 0 {
			return "0.00"
		}
		return dec2(net.Div(decimal.NewFromInt(int64(n))))
	}
	out := CashierReport{From: fs, To: ts, Rows: make([]CashierRow, 0, len(rows))}
	var gross, disc, net decimal.Decimal
	for _, r := range rows {
		out.Rows = append(out.Rows, CashierRow{Cashier: r.Cashier, Count: r.Count, Gross: dec2(r.Gross), Discount: dec2(r.Discount), Net: dec2(r.Net), AvgBasket: avg(r.Net, r.Count)})
		out.Count += r.Count
		gross, disc, net = gross.Add(r.Gross), disc.Add(r.Discount), net.Add(r.Net)
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Cashier, strconv.Itoa(r.Count), r.Gross, r.Discount, r.AvgBasket, r.Net})
		}
		return writeSheet(c, "sales_by_cashier_"+fs+"_"+ts, []string{"Cashier", "Sales", "Gross", "Discount", "Avg basket", "Net"}, sheet)
	}
	out.Gross, out.Discount, out.Net, out.AvgBasket = dec2(gross), dec2(disc), dec2(net), avg(net, out.Count)
	return c.JSON(http.StatusOK, out)
}
