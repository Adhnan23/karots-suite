package api

import (
	"net/http"

	"karots-suite/internal/features/reports"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminFinance(g *echo.Group) {
	g.GET("/finance", h.AdminFinance)
}

// financeGrain picks the trend's period for a range: days up to a month, weeks
// up to half a year, months beyond.
func financeGrain(days int) string {
	switch {
	case days <= 31:
		return "day"
	case days <= 183:
		return "week"
	}
	return "month"
}

// AdminFinance is the Finance hub's one read (Overview / Profit /
// Cash). Nothing chosen = this month.
func (h *handlers) AdminFinance(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	// One moment for the whole page (RPT-36): the trend and categories add up to its P&L.
	grain := financeGrain(int(to.Sub(from).Hours() / 24))
	var pl *reports.PL
	var trend []reports.DayRow
	var cats []reports.CategoryProfit
	if err := reports.NewService(h.d.DB).Snapshot(ctx, func(rep *reports.Service) (err error) {
		if pl, err = rep.Compute(ctx, from, to); err != nil {
			return err
		}
		if trend, err = rep.SalesByPeriod(ctx, from, to, grain); err != nil {
			return err
		}
		cats, err = rep.ProfitByCategory(ctx, from, to)
		return err
	}); err != nil {
		return err
	}
	// Plugin income (core stays plugin-free: the hooks are read here) adds to
	// net profit, one line each; nothing registers in a core-only build.
	net := pl.NetProfit
	out := AdminFinance{From: fs, To: ts, Grain: grain, SalesCount: pl.SalesCount, PluginIncome: []FinanceLine{},
		GrossRevenue: dec2(pl.GrossRevenue), Returns: dec2(pl.Returns), Revenue: dec2(pl.Revenue), COGS: dec2(pl.COGS),
		GrossProfit: dec2(pl.GrossProfit), GrossMargin: pl.GrossMargin.StringFixed(1), Expenses: dec2(pl.Expenses), Losses: dec2(pl.Losses),
		OwnUse: dec2(pl.OwnUse), StaffWelfare: dec2(pl.StaffWelfare), StockCorrection: dec2(pl.StockCorrections), Recoveries: dec2(pl.Recoveries),
		OtherIncome: dec2(pl.OtherIncome), DiscountsReceived: dec2(pl.DiscountsReceived), ZeroCostLines: pl.ZeroCostLines, ZeroCostRevenue: dec2(pl.ZeroCostRevenue),
		Received: dec2(pl.Received), Tax: dec2(pl.Tax), CashWithdrawn: dec2(pl.CashWithdrawn), RegisterDiff: dec2(pl.RegisterDiff),
		Receivables: dec2(pl.Receivables), Advances: dec2(pl.CustomerAdvances), Payables: dec2(pl.Payables), SupplierCredit: dec2(pl.SupplierCredit),
		Trend: []FinancePoint{}, ByPayment: []FinanceSlice{}, Categories: []FinanceCategory{}, TopProducts: []FinanceProduct{}}
	for _, s := range plugin.PLIncomes() {
		amt, err := s.Amount(ctx, from, to)
		if err != nil || amt.IsZero() {
			continue
		}
		out.PluginIncome = append(out.PluginIncome, FinanceLine{Label: s.Label, Amount: dec2(amt)})
		net = net.Add(amt)
	}
	out.NetProfit = dec2(net)
	if wantsSheet(c) {
		return financeSheet(c, out)
	}
	for _, r := range trend {
		out.Trend = append(out.Trend, FinancePoint{Day: r.Day.Format("2006-01-02"), Revenue: dec2(r.Revenue), Profit: dec2(r.Profit)})
	}
	for _, m := range pl.ByPayment {
		out.ByPayment = append(out.ByPayment, FinanceSlice{Method: m.Method, Count: m.Count, Total: dec2(m.Total)})
	}
	for _, r := range cats {
		out.Categories = append(out.Categories, FinanceCategory{Category: r.Category, Revenue: dec2(r.Revenue), Profit: dec2(r.Profit)})
	}
	for _, p := range pl.TopProducts {
		out.TopProducts = append(out.TopProducts, FinanceProduct{Name: p.ProductName, Qty: p.Qty.String(), Revenue: dec2(p.Revenue), Profit: dec2(p.Profit)})
	}
	return c.JSON(http.StatusOK, out)
}

// financeSheet is the P&L as a download:
// the statement, plugin income, then the balances either way.
func financeSheet(c echo.Context, f AdminFinance) error {
	rows := [][]string{
		{"Gross revenue (before tax)", f.GrossRevenue}, {"Returns", f.Returns}, {"Net revenue", f.Revenue}, {"Cost of goods sold", f.COGS},
		{"Gross profit", f.GrossProfit}, {"Expenses", f.Expenses}, {"Losses (damage, warranty)", f.Losses}, {"Own use", f.OwnUse},
		{"Staff welfare", f.StaffWelfare}, {"Stock corrections", f.StockCorrection}, {"Till over / short", f.RegisterDiff},
		{"Revenue sold at no recorded cost", f.ZeroCostRevenue}, {"Recovered from suppliers", f.Recoveries}, {"Discounts from suppliers", f.DiscountsReceived}, {"Other income (interest)", f.OtherIncome},
	}
	for _, l := range f.PluginIncome {
		rows = append(rows, []string{l.Label, l.Amount})
	}
	rows = append(rows, []string{"Net profit", f.NetProfit}, []string{"Received at sale", f.Received}, []string{"Tax collected (owed to the tax office)", f.Tax}, []string{"Receivables", f.Receivables},
		[]string{"Customer advances (owed back)", f.Advances}, []string{"Payables", f.Payables}, []string{"Supplier credit (they owe us)", f.SupplierCredit})
	return writeSheet(c, "finance_"+f.From+"_"+f.To, []string{"Line", "Amount"}, rows)
}
