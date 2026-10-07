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
