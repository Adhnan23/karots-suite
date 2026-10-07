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

// SalesReport is the Sales report's head (karots-pos SalesReport): this range,
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

// PeakHoursReport is sales by weekday and hour (karots-pos Peak Hours). Only
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

// ProductSalesReport is one product over time (karots-pos Product Sales).
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
