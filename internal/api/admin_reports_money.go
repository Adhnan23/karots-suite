package api

import (
	"net/http"
	"sort"
	"strconv"

	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsMoney(g *echo.Group) {
	g.GET("/reports/profit-by-category", h.ReportCategoryProfit)
	g.GET("/reports/expenses", h.ReportExpenses)
	g.GET("/reports/tax", h.ReportTax)
}

// ReportCategoryProfit is Profit by category: each category's
// own sales plus its descendants', as a tree; ?category= shows one branch.
// The totals add each category's own lines once, so a parent never doubles.
func (h *handlers) ReportCategoryProfit(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rep := reports.NewService(h.d.DB)
	own, err := rep.ProfitByCategory(ctx, from, to)
	if err != nil {
		return err
	}
	cats, err := rep.CategoriesFlat(ctx)
	if err != nil {
		return err
	}
	parent := map[int64]*int64{}
	name := map[int64]string{}
	kids := map[int64][]int64{}
	var roots []int64
	for _, k := range cats {
		parent[k.ID], name[k.ID] = k.ParentID, k.Name
		if k.ParentID == nil {
			roots = append(roots, k.ID)
		} else {
			kids[*k.ParentID] = append(kids[*k.ParentID], k.ID)
		}
	}
	type sums struct{ rev, cogs, profit decimal.Decimal }
	sub := map[int64]*sums{}
	add := func(id int64, r reports.CategoryProfit) {
		for at := &id; at != nil; at = parent[*at] {
			if sub[*at] == nil {
				sub[*at] = &sums{}
			}
			x := sub[*at]
			x.rev, x.cogs, x.profit = x.rev.Add(r.Revenue), x.cogs.Add(r.COGS), x.profit.Add(r.Profit)
		}
	}
	for _, r := range own {
		add(r.CatID, r)
	}
	if id, _ := strconv.ParseInt(c.QueryParam("category"), 10, 64); id > 0 {
		roots = []int64{id}
	}
	out := CategoryProfitReport{From: fs, To: ts, Nodes: []CategoryProfitNode{}}
	var walk func(ids []int64, depth int)
	walk = func(ids []int64, depth int) {
		ids = append([]int64(nil), ids...)
		// Siblings by profit, biggest first; categories with no sales (skipped
		// below) sort last.
		has := func(id int64) bool { return sub[id] != nil }
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := ids[i], ids[j]
			if has(a) != has(b) {
				return has(a)
			}
			return has(a) && sub[a].profit.GreaterThan(sub[b].profit)
		})
		for _, id := range ids {
			x := sub[id]
			if x == nil {
				continue
			}
			out.Nodes = append(out.Nodes, CategoryProfitNode{ID: id, ParentID: parent[id], Name: name[id], Depth: depth,
				Revenue: dec2(x.rev), COGS: dec2(x.cogs), Profit: dec2(x.profit), Margin: pct1(x.profit, x.rev)})
			walk(kids[id], depth+1)
		}
	}
	walk(roots, 0)
	var rev, cogs, profit decimal.Decimal
	for _, r := range roots {
		if x := sub[r]; x != nil {
			rev, cogs, profit = rev.Add(x.rev), cogs.Add(x.cogs), profit.Add(x.profit)
		}
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Nodes))
		for _, n := range out.Nodes {
			sheet = append(sheet, []string{n.Name, strconv.Itoa(n.Depth), n.Revenue, n.COGS, n.Profit, n.Margin})
		}
		return writeSheet(c, "profit_by_category_"+fs+"_"+ts, []string{"Category", "Level", "Revenue", "COGS", "Profit", "Margin %"}, sheet)
	}
	out.Revenue, out.COGS, out.Profit, out.Margin = dec2(rev), dec2(cogs), dec2(profit), pct1(profit, rev)
	return c.JSON(http.StatusOK, out)
}

// ReportExpenses is Expenses by category, biggest first — the
// detail behind the P&L's one expenses line.
func (h *handlers) ReportExpenses(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	rows, err := reports.NewService(h.d.DB).ExpensesByCategory(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := ExpensesReport{From: fs, To: ts, Rows: make([]ExpenseCatTotal, 0, len(rows))}
	var total decimal.Decimal
	for _, r := range rows {
		out.Rows = append(out.Rows, ExpenseCatTotal{Category: r.Category, Count: r.Count, Total: dec2(r.Total)})
		out.Count += r.Count
		total = total.Add(r.Total)
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Category, strconv.Itoa(r.Count), r.Total})
		}
		return writeSheet(c, "expenses_by_category_"+fs+"_"+ts, []string{"Category", "Count", "Total"}, sheet)
	}
	out.Total = dec2(total)
	return c.JSON(http.StatusOK, out)
}

// ReportTax is tax charged at sale per shop day.
func (h *handlers) ReportTax(c echo.Context) error {
	from, to, fs, ts, err := reportRange(c)
	if err != nil {
		return err
	}
	sum, err := reports.NewService(h.d.DB).TaxSummary(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	out := TaxReport{From: fs, To: ts, Base: dec2(sum.TotalBase), Tax: dec2(sum.TotalTax), Rows: make([]TaxDay, 0, len(sum.Rows))}
	for _, r := range sum.Rows {
		out.Rows = append(out.Rows, TaxDay{Day: r.Day.Format("2006-01-02"), Count: r.Count, Base: dec2(r.Base), Tax: dec2(r.Tax)})
		out.Count += r.Count
	}
	if wantsSheet(c) {
		sheet := make([][]string, 0, len(out.Rows))
		for _, r := range out.Rows {
			sheet = append(sheet, []string{r.Day, strconv.Itoa(r.Count), r.Base, r.Tax})
		}
		return writeSheet(c, "tax_"+fs+"_"+ts, []string{"Date", "Sales", "Taxable base", "Tax"}, sheet)
	}
	return c.JSON(http.StatusOK, out)
}
