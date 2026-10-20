package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/datetime"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminDashboard(g *echo.Group) {
	g.GET("/dashboard", h.AdminDashboard)
	g.GET("/dashboard/charts", h.AdminDashboardCharts)
}

// shopDay is midnight of t's day in shop time.
func shopDay(t time.Time) time.Time {
	t = t.In(datetime.Location)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, datetime.Location)
}

func dashboardDay(pl *reports.PL) DashboardDay {
	d := DashboardDay{Count: pl.SalesCount, Revenue: dec2(pl.Revenue), Profit: dec2(pl.GrossProfit), Margin: pl.GrossMargin.StringFixed(1), AvgBasket: "0.00"}
	if pl.SalesCount > 0 {
		d.AvgBasket = dec2(pl.Revenue.Div(decimal.NewFromInt(int64(pl.SalesCount))))
	}
	return d
}

// periods is every period start from `from` (aligned to the grain) up to
// `to`, with revenue / profit filled from rows and zeros between — so two
// ranges line up period by period.
func periods(rows []reports.DayRow, from, to time.Time, grain string) []FinancePoint {
	got := map[string]reports.DayRow{}
	for _, r := range rows {
		got[r.Day.Format("2006-01-02")] = r
	}
	step := func(t time.Time) time.Time {
		if grain == "week" {
			return t.AddDate(0, 0, 7)
		}
		return t.AddDate(0, 0, 1)
	}
	start := shopDay(from)
	if grain == "week" {
		start = start.AddDate(0, 0, -((int(start.Weekday()) + 6) % 7))
	}
	var out []FinancePoint
	for t := start; t.Before(to); t = step(t) {
		k := t.Format("2006-01-02")
		r := got[k]
		out = append(out, FinancePoint{Day: k, Revenue: dec2(r.Revenue), Profit: dec2(r.Profit)})
	}
	return out
}

// AdminDashboard is the admin home's data (karots-pos Dashboard). Today and
// yesterday are shop days; every figure counts as the P&L does.
func (h *handlers) AdminDashboard(c echo.Context) error {
	ctx := c.Request().Context()
	today := shopDay(time.Now())
	tomorrow, yesterday := today.AddDate(0, 0, 1), today.AddDate(0, 0, -1)
	rep := reports.NewService(h.d.DB)
	pl, err := rep.Compute(ctx, today, tomorrow)
	if err != nil {
		return err
	}
	ypl, err := rep.Compute(ctx, yesterday, today)
	if err != nil {
		return err
	}
	sparkFrom := today.AddDate(0, 0, -13)
	spark, err := rep.SalesByPeriod(ctx, sparkFrom, tomorrow, "day")
	if err != nil {
		return err
	}
	out := Dashboard{Today: dashboardDay(pl), Yesterday: dashboardDay(ypl), Spark: periods(spark, sparkFrom, tomorrow, "day"),
		Owed: dec2(pl.Receivables), Paid: make([]FinanceSlice, 0, len(pl.ByPayment)), Recent: []AdminSaleRow{}}
	for _, m := range pl.ByPayment {
		out.Paid = append(out.Paid, FinanceSlice{Method: m.Method, Count: m.Count, Total: dec2(m.Total)})
	}
	tills, err := h.d.CashRegister.OpenSessions(ctx)
	if err != nil {
		return err
	}
	cash := decimal.Zero
	for _, t := range tills {
		if s, err := h.d.CashRegister.Summary(ctx, t.UserID); err == nil {
			cash = cash.Add(s.Expected)
		} else {
			cash = cash.Add(t.OpeningCash)
		}
	}
	out.TillCash, out.OpenTills = dec2(cash), len(tills)
	if out.Attention, err = h.attention(ctx); err != nil {
		return err
	}
	if middleware.CurrentRole(c) != auth.RoleAdmin {
		out.Attention.BackupCopies = []string{} // the Data page is the admin's
	}
	_ = h.d.DB.GetContext(ctx, &out.Owing, `SELECT COUNT(*) FROM customers WHERE outstanding_balance > 0`)
	recent, err := h.d.Sales.List(ctx, sales.ListFilter{From: &today, Limit: 8})
	if err != nil {
		return err
	}
	for _, s := range recent {
		out.Recent = append(out.Recent, AdminSaleRow{SaleSummary: saleSummary(s), Paid: dec2(s.PaidAmount.Sub(s.ChangeGiven)), COGS: dec2(s.COGS), Profit: dec2(s.Profit), Margin: s.Margin.StringFixed(1)})
	}
	return c.JSON(http.StatusOK, out)
}

// attention counts what needs someone: low stock, lots expired or expiring
// within 30 days, quick-added items to review, customers over their limit.
func (h *handlers) attention(ctx context.Context) (DashboardAttention, error) {
	var a DashboardAttention
	_, low, err := h.t.products.List(ctx, products.ListQuery{LowStock: true, Limit: 1})
	if err != nil {
		return a, err
	}
	a.LowStock = low
	lots, err := h.t.stock.Expiring(ctx, 30)
	if err != nil {
		return a, err
	}
	now := time.Now()
	for _, b := range lots {
		if b.ExpiryDate != nil && b.ExpiryDate.Before(now) {
			a.Expired++
		} else {
			a.Expiring++
		}
	}
	if a.Review, err = h.t.products.CountNeedsReview(ctx); err != nil {
		return a, err
	}
	var over struct {
		N   int             `db:"n"`
		Sum decimal.Decimal `db:"sum"`
	}
	_ = h.d.DB.GetContext(ctx, &over, `SELECT COUNT(*) AS n, COALESCE(SUM(outstanding_balance),0) AS sum FROM customers
		WHERE is_active AND credit_limit > 0 AND outstanding_balance > credit_limit`)
	a.OverLimit, a.OverLimitSum = over.N, dec2(over.Sum)
	copies, err := backup.Copies(ctx, h.d.DB)
	if err != nil {
		return a, err
	}
	a.BackupCopies = staleCopies(copies, now)
	a.StaleTills, a.NoTillSales = []StaleTill{}, []NoTillSale{}
	open, err := h.d.CashRegister.OpenSessions(ctx)
	if err != nil {
		return a, err
	}
	for _, s := range open {
		if cashregister.Stale(s.OpenedAt, now) {
			a.StaleTills = append(a.StaleTills, StaleTill{ID: s.ID, UserID: s.UserID, Cashier: s.UserName, OpenedAt: s.OpenedAt.Format(time.RFC3339)})
		}
	}
	// Cash a till never saw: no session of that cashier spans the sale (never refused — flagged).
	var orphans []struct {
		ID        int64           `db:"id"`
		ReceiptNo string          `db:"receipt_no"`
		Cashier   string          `db:"cashier"`
		At        time.Time       `db:"created_at"`
		Cash      decimal.Decimal `db:"cash"`
	}
	if err := h.d.DB.SelectContext(ctx, &orphans, `
		SELECT s.id, s.receipt_no, u.name AS cashier, s.created_at,
		       (SELECT SUM(p.amount) FROM payments p WHERE p.sale_id = s.id AND p.method = 'cash') - s.change_given AS cash
		FROM sales s JOIN users u ON u.id = s.cashier_id
		WHERE s.created_at >= $1 AND s.status <> 'void'
		  AND EXISTS (SELECT 1 FROM payments p WHERE p.sale_id = s.id AND p.method = 'cash')
		  AND NOT EXISTS (SELECT 1 FROM cash_register cr WHERE cr.user_id = s.cashier_id
		                  AND cr.opened_at <= s.created_at AND (cr.closed_at IS NULL OR cr.closed_at >= s.created_at))
		ORDER BY s.created_at DESC LIMIT 20`, shopDay(now)); err != nil {
		return a, err
	}
	for _, o := range orphans {
		if o.Cash.IsPositive() {
			a.NoTillSales = append(a.NoTillSales, NoTillSale{ID: o.ID, ReceiptNo: o.ReceiptNo, Cashier: o.Cashier, At: o.At.Format(time.RFC3339), Cash: dec2(o.Cash)})
		}
	}
	return a, nil
}

// AdminDashboardCharts is the chart range (?days= 7 | 14 | 30 | 90, default
// 14): revenue by day (by week past a month) against the period before, and
// the top five sellers.
func (h *handlers) AdminDashboardCharts(c echo.Context) error {
	ctx := c.Request().Context()
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days != 7 && days != 30 && days != 90 {
		days = 14
	}
	grain := "day"
	if days > 31 {
		grain = "week"
	}
	to := shopDay(time.Now()).AddDate(0, 0, 1)
	from := to.AddDate(0, 0, -days)
	pfrom := from.AddDate(0, 0, -days)
	rep := reports.NewService(h.d.DB)
	cur, err := rep.SalesByPeriod(ctx, from, to, grain)
	if err != nil {
		return err
	}
	prev, err := rep.SalesByPeriod(ctx, pfrom, from, grain)
	if err != nil {
		return err
	}
	top, err := rep.TopProducts(ctx, from, to, "revenue", 5)
	if err != nil {
		return err
	}
	out := DashboardCharts{Days: days, Grain: grain, Cur: periods(cur, from, to, grain), Prev: periods(prev, pfrom, from, grain), Top: make([]TopProductRow, 0, len(top))}
	if n := len(out.Cur); len(out.Prev) > n { // a week grain can start one bucket early
		out.Prev = out.Prev[len(out.Prev)-n:]
	}
	for _, r := range top {
		out.Top = append(out.Top, TopProductRow{ProductID: r.ProductID, Name: r.ProductName, Qty: r.Qty.String(), Revenue: dec2(r.Revenue), Profit: dec2(r.Profit), Margin: pct1(r.Profit, r.Revenue)})
	}
	return c.JSON(http.StatusOK, out)
}
