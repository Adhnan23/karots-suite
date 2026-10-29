package api

// ---- Reports (M5): /api/v1/admin/reports/* ----

// SalesReportSum is the headline over every sale the filters match (SQL).
// Revenue is net of pass-through face and returns (the P&L's revenue); Total
// is the sale totals, which the average basket divides.
type SalesReportSum struct {
	Count     int    `json:"count"`
	Revenue   string `json:"revenue"`
	Total     string `json:"total"`
	Discount  string `json:"discount"`
	COGS      string `json:"cogs"`
	Profit    string `json:"profit"`
	Margin    string `json:"margin"` // percent, 1 dp
	AvgBasket string `json:"avgBasket"`
}

// SalesReport is the Sales report's head: this range,
// the equal-length range before it (same filters) for the ▲▼ deltas, and the
// trend by Grain. The receipts are the admin sales list's pages.
type SalesReport struct {
	From  string         `json:"from"`
	To    string         `json:"to"`
	Grain string         `json:"grain"`
	Sum   SalesReportSum `json:"sum"`
	Prev  SalesReportSum `json:"prev"`
	Trend []FinancePoint `json:"trend"`
}

// PeakCell is one weekday × hour cell in shop time. Dow 0 = Sunday.
type PeakCell struct {
	Dow     int    `json:"dow"`
	Hour    int    `json:"hour"`
	Count   int    `json:"count"`
	Revenue string `json:"revenue"`
}

// PeakHoursReport is sales by weekday and hour. Only
// cells with sales are sent.
type PeakHoursReport struct {
	From    string     `json:"from"`
	To      string     `json:"to"`
	Count   int        `json:"count"`
	Revenue string     `json:"revenue"`
	Cells   []PeakCell `json:"cells"`
}

// TopProductRow is one product's net units, revenue and profit in range.
type TopProductRow struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Qty       string `json:"qty"`
	Revenue   string `json:"revenue"`
	Profit    string `json:"profit"`
	Margin    string `json:"margin"` // percent, 1 dp
}

// TopProductsReport ranks products by Order (revenue | qty | profit). The
// totals cover every product sold in range, not just the ranked rows, so a
// row's share and the Pareto line are of the whole.
type TopProductsReport struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Order    string          `json:"order"`
	Products int             `json:"products"`
	Qty      string          `json:"qty"`
	Revenue  string          `json:"revenue"`
	Profit   string          `json:"profit"`
	Margin   string          `json:"margin"`
	Rows     []TopProductRow `json:"rows"`
}

// ProductPeriod is one period of one product, with the same period a year
// earlier beside it.
type ProductPeriod struct {
	Day             string `json:"day"`
	Qty             string `json:"qty"`
	Revenue         string `json:"revenue"`
	LastYearQty     string `json:"lastYearQty"`
	LastYearRevenue string `json:"lastYearRevenue"`
}

// ProductSalesReport is one product over time.
// ProductID 0 = none picked yet (empty rows).
type ProductSalesReport struct {
	From        string          `json:"from"`
	To          string          `json:"to"`
	Grain       string          `json:"grain"`
	ProductID   int64           `json:"productId"`
	Name        string          `json:"name"`
	Qty         string          `json:"qty"`
	Revenue     string          `json:"revenue"`
	LastYearQty string          `json:"lastYearQty"`
	Rows        []ProductPeriod `json:"rows"`
}

// CashierRow is one cashier's takings in range (gross before discounts, net
// after; pass-through face value excluded).
type CashierRow struct {
	Cashier   string `json:"cashier"`
	Count     int    `json:"count"`
	Gross     string `json:"gross"`
	Discount  string `json:"discount"`
	Net       string `json:"net"`
	AvgBasket string `json:"avgBasket"`
}

// CashierReport is Sales by cashier, biggest takings first.
type CashierReport struct {
	From      string       `json:"from"`
	To        string       `json:"to"`
	Count     int          `json:"count"`
	Gross     string       `json:"gross"`
	Discount  string       `json:"discount"`
	Net       string       `json:"net"`
	AvgBasket string       `json:"avgBasket"`
	Rows      []CashierRow `json:"rows"`
}

// ReturnTop is one of the most-returned products in range.
type ReturnTop struct {
	Name   string `json:"name"`
	Qty    string `json:"qty"`
	Refund string `json:"refund"`
}

// ReturnsReport is the Returns report's head: every return MADE in range (by
// the return's date), what went back, and the ten most-returned products. Its
// lines page from /reports/returns/lines.
type ReturnsReport struct {
	From     string      `json:"from"`
	To       string      `json:"to"`
	Returns  int         `json:"returns"`
	Receipts int         `json:"receipts"`
	Qty      string      `json:"qty"`
	Refund   string      `json:"refund"`
	Top      []ReturnTop `json:"top"`
}

// ReturnLineRow is one returned line. When is the return; SaleDate (shop
// date) is the sale it came from.
type ReturnLineRow struct {
	When      string `json:"when"`
	SaleID    int64  `json:"saleId"`
	ReceiptNo string `json:"receiptNo"`
	SaleDate  string `json:"saleDate"`
	Product   string `json:"product"`
	Qty       string `json:"qty"`
	Refund    string `json:"refund"`
	Customer  string `json:"customer"`
	Reason    string `json:"reason"`
}

// DueRow is one customer who owes us, or one supplier we owe. Terms is the
// credit limit (customers) or "N days" credit (suppliers); Flag = over the
// limit / past the supplier's credit days.
type DueRow struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Balance  string `json:"balance"`
	Terms    string `json:"terms"`
	Oldest   string `json:"oldest"` // oldest unpaid credit sale / bill, shop date; "" when none
	AgeDays  int    `json:"ageDays"`
	Flag     bool   `json:"flag"`
	Disabled bool   `json:"disabled"`
}

// DuesReport is Customer dues or Supplier dues: who owes, aged by
// the oldest unpaid credit sale / bill into 0–30 / 31–60 / 61–90 / 90+ days.
type DuesReport struct {
	Count      int       `json:"count"`
	Total      string    `json:"total"`
	Flagged    int       `json:"flagged"`
	FlaggedSum string    `json:"flaggedSum"`
	OldestDays int       `json:"oldestDays"`
	Buckets    [4]string `json:"buckets"`
	Rows       []DueRow  `json:"rows"`
}

// SpendRow is what was bought from one supplier in range, net of goods sent
// back.
type SpendRow struct {
	SupplierID int64  `json:"supplierId"`
	Supplier   string `json:"supplier"`
	Orders     int    `json:"orders"`
	Spend      string `json:"spend"`
	Paid       string `json:"paid"`
	Due        string `json:"due"`
}

// SupplierSpendReport ranks suppliers by spend.
type SupplierSpendReport struct {
	From   string     `json:"from"`
	To     string     `json:"to"`
	Orders int        `json:"orders"`
	Spend  string     `json:"spend"`
	Paid   string     `json:"paid"`
	Due    string     `json:"due"`
	Rows   []SpendRow `json:"rows"`
}

// PurchaseReportRow is one received delivery. Due is what's left after
// payments and goods credited back.
type PurchaseReportRow struct {
	ID       int64  `json:"id"`
	When     string `json:"when"`
	Supplier string `json:"supplier"`
	Invoice  string `json:"invoice"`
	Total    string `json:"total"`
	Paid     string `json:"paid"`
	Credited string `json:"credited"`
	Due      string `json:"due"`
}

// PurchasesReport is the deliveries received in range.
type PurchasesReport struct {
	From     string              `json:"from"`
	To       string              `json:"to"`
	Count    int                 `json:"count"`
	Total    string              `json:"total"`
	Paid     string              `json:"paid"`
	Credited string              `json:"credited"`
	Due      string              `json:"due"`
	Rows     []PurchaseReportRow `json:"rows"`
}

// CategoryProfitNode is one category with its whole subtree's sales (a parent
// includes its children's). Depth 0 = the top of what's shown.
type CategoryProfitNode struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parentId"`
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	Revenue  string `json:"revenue"`
	COGS     string `json:"cogs"`
	Profit   string `json:"profit"`
	Margin   string `json:"margin"`
}

// CategoryProfitReport is Profit by category: the tree in
// display order (siblings by profit), the totals counting each line once.
type CategoryProfitReport struct {
	From    string               `json:"from"`
	To      string               `json:"to"`
	Revenue string               `json:"revenue"`
	COGS    string               `json:"cogs"`
	Profit  string               `json:"profit"`
	Margin  string               `json:"margin"`
	Nodes   []CategoryProfitNode `json:"nodes"`
}

// ExpenseCatTotal is one expense category's total in range.
type ExpenseCatTotal struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
	Total    string `json:"total"`
}

// ExpensesReport is Expenses by category, biggest first.
type ExpensesReport struct {
	From  string            `json:"from"`
	To    string            `json:"to"`
	Count int               `json:"count"`
	Total string            `json:"total"`
	Rows  []ExpenseCatTotal `json:"rows"`
}

// TaxDay is one shop day's tax charged at sale.
type TaxDay struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
	Base  string `json:"base"`
	Tax   string `json:"tax"`
}

// TaxReport is the Tax summary.
type TaxReport struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Count int      `json:"count"`
	Base  string   `json:"base"`
	Tax   string   `json:"tax"`
	Rows  []TaxDay `json:"rows"`
}

// TillTotals is every till session the Till sessions filters match (SQL, not
// the page): the Cash Register report's totals. Expected / counted
// / over-short cover closed sessions.
type TillTotals struct {
	Sessions  int    `json:"sessions"`
	Opening   string `json:"opening"`
	Expected  string `json:"expected"`
	Counted   string `json:"counted"`
	OverShort string `json:"overShort"`
	Short     int    `json:"short"`
}
