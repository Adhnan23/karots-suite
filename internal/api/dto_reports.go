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

// DuesReport is Customer dues or Supplier dues (karots-pos): who owes, aged by
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

// SupplierSpendReport ranks suppliers by spend (karots-pos Spend by Supplier).
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

// PurchasesReport is the deliveries received in range (karots-pos Purchases).
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

// CategoryProfitReport is Profit by category (karots-pos): the tree in
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

// ExpensesReport is Expenses by category (karots-pos), biggest first.
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

// TaxReport is the Tax summary (karots-pos, tax-registered shops only).
type TaxReport struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Count int      `json:"count"`
	Base  string   `json:"base"`
	Tax   string   `json:"tax"`
	Rows  []TaxDay `json:"rows"`
}

// TillTotals is every till session the Till sessions filters match (SQL, not
// the page): the karots-pos Cash Register report's totals. Expected / counted
// / over-short cover closed sessions.
type TillTotals struct {
	Sessions  int    `json:"sessions"`
	Opening   string `json:"opening"`
	Expected  string `json:"expected"`
	Counted   string `json:"counted"`
	OverShort string `json:"overShort"`
	Short     int    `json:"short"`
}

// UnitAmount is a quantity in one unit ("120 kg").
type UnitAmount struct {
	Unit string `json:"unit"`
	Qty  string `json:"qty"`
}

// InventoryNode is one row of the category breakdown: a child category over
// its whole subtree, or (Direct) the products filed on the category itself.
type InventoryNode struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Direct      bool   `json:"direct"`
	HasChildren bool   `json:"hasChildren"`
	SKUs        int    `json:"skus"`
	InStock     int    `json:"inStock"`
	CostValue   string `json:"costValue"`
	RetailValue string `json:"retailValue"`
}

// Crumb is one step of the path from the whole shop down to a category.
type Crumb struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// InventoryReport is the stock on hand valued at cost (by its lots) and at
// retail for one branch of the catalogue (karots-pos Inventory Valuation).
// Its products page from /reports/inventory/products.
type InventoryReport struct {
	SKUs        int             `json:"skus"`
	InStock     int             `json:"inStock"`
	Units       []UnitAmount    `json:"units"`
	CostValue   string          `json:"costValue"`
	RetailValue string          `json:"retailValue"`
	Margin      string          `json:"margin"`
	MarginPct   string          `json:"marginPct"`
	MissingCost int             `json:"missingCost"`
	NegativeQty int             `json:"negativeQty"`
	Breadcrumb  []Crumb         `json:"breadcrumb"`
	Children    []InventoryNode `json:"children"`
}

// InventoryProduct is one product's stock and what it's worth.
type InventoryProduct struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Barcode     string `json:"barcode"`
	Category    string `json:"category"`
	Unit        string `json:"unit"`
	Qty         string `json:"qty"`
	CostValue   string `json:"costValue"`
	RetailValue string `json:"retailValue"`
}

// CountSheetLine is one product to count on the shelf.
type CountSheetLine struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Barcode  string `json:"barcode"`
	Unit     string `json:"unit"`
	Category string `json:"category"` // "Drinks > Soft drinks"
	OnHand   string `json:"onHand"`
}

// CountSheet is the physical-count worklist (karots-pos Stock Count Sheet):
// active stocked products in a category (any depth) and / or from a supplier,
// grouped by category path. Category / Supplier name the filters.
type CountSheet struct {
	Category string           `json:"category"`
	Supplier string           `json:"supplier"`
	Rows     []CountSheetLine `json:"rows"`
}

// ReadinessLine is one thing a plugin says needs topping up.
type ReadinessLine struct {
	Name   string `json:"name"`
	Have   string `json:"have"`
	Target string `json:"target"`
	Need   string `json:"need"`
}

// ReadinessGroup is one plugin's readiness section (e.g. a reload float).
type ReadinessGroup struct {
	Title string          `json:"title"`
	Need  string          `json:"need"`
	Rows  []ReadinessLine `json:"rows"`
}

// TopupStockRow is one product at or under its reorder level and what it
// costs to bring it back up.
type TopupStockRow struct {
	ProductID int64  `json:"productId"`
	Category  string `json:"category"`
	Product   string `json:"product"`
	OnHand    string `json:"onHand"`
	Reorder   int    `json:"reorder"`
	Short     string `json:"short"`
	Cost      string `json:"cost"`
	Restock   string `json:"restock"`
}

// PlaceBalance is money in one place now.
type PlaceBalance struct {
	Name    string `json:"name"`
	Balance string `json:"balance"`
}

// TopupReport is Top-up & reorder (karots-pos): what must be topped up (plugin
// floats, stock under its level) against the money on hand to pay for it.
type TopupReport struct {
	Plugins       []ReadinessGroup `json:"plugins"`
	PluginNeed    string           `json:"pluginNeed"`
	Stock         []TopupStockRow  `json:"stock"`
	StockNeed     string           `json:"stockNeed"`
	TotalNeed     string           `json:"totalNeed"`
	Physical      []PlaceBalance   `json:"physical"`
	PhysicalTotal string           `json:"physicalTotal"`
	Bank          []PlaceBalance   `json:"bank"`
	BankTotal     string           `json:"bankTotal"`
	Available     string           `json:"available"`
	Gap           string           `json:"gap"` // available − need; negative = short
}

// VarianceLine is one ingredient: what recipes say was used against what the
// stock actually moved. Drift is Diff as a % of expected (1 dp).
type VarianceLine struct {
	Ingredient string `json:"ingredient"`
	Unit       string `json:"unit"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Diff       string `json:"diff"`
	Drift      string `json:"drift"`
}

// RecipeVarianceReport (karots-pos Recipe Variance). Drifting counts the
// ingredients more than 10% off, over every row.
type RecipeVarianceReport struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Drifting int            `json:"drifting"`
	Rows     []VarianceLine `json:"rows"`
}

// ServiceLine is one service's takings, what its recipe consumed, the
// expenses booked against it, and what's left.
type ServiceLine struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Units     string `json:"units"`
	Revenue   string `json:"revenue"`
	COGS      string `json:"cogs"`
	Gross     string `json:"gross"`
	Expenses  string `json:"expenses"`
	Net       string `json:"net"`
	Margin    string `json:"margin"`
}

// ServiceProfitReport (karots-pos Service Profit): did each counter pay for itself.
type ServiceProfitReport struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	Revenue  string        `json:"revenue"`
	Expenses string        `json:"expenses"`
	Net      string        `json:"net"`
	Rows     []ServiceLine `json:"rows"`
}

// RecoveryOutcome is one outcome's tally in range.
type RecoveryOutcome struct {
	Outcome   string `json:"outcome"`
	Count     int    `json:"count"`
	LossValue string `json:"lossValue"`
	Recovered string `json:"recovered"`
}

// LossesReport is Losses & Recovery's head (karots-pos): stock lost to damage
// and warranty replacements in range, against what suppliers made good.
type LossesReport struct {
	From          string            `json:"from"`
	To            string            `json:"to"`
	DamageCount   int               `json:"damageCount"`
	DamageCost    string            `json:"damageCost"`
	WarrantyCount int               `json:"warrantyCount"`
	WarrantyCost  string            `json:"warrantyCost"`
	Recovered     string            `json:"recovered"`
	Net           string            `json:"net"` // lost − recovered
	Outcomes      []RecoveryOutcome `json:"outcomes"`
}

// LossRow is one loss in range. SourceID 0 = its claim can't be traced, so it
// can't be recovered; Outcome "" = not recovered yet.
type LossRow struct {
	Kind      string `json:"kind"`
	SourceID  int64  `json:"sourceId"`
	ID        int64  `json:"id"`
	Product   string `json:"product"`
	Qty       string `json:"qty"`
	Loss      string `json:"loss"`
	Note      string `json:"note"`
	By        string `json:"by"`
	At        string `json:"at"`
	Outcome   string `json:"outcome"`
	Recovered string `json:"recovered"`
}

// RecoveryRequest books what a supplier made good on one loss (karots-pos
// RecoveryRecord): replacement (restocked) | paid (credited against what we
// owe them — cash handed over is a supplier refund instead) | written_off.
type RecoveryRequest struct {
	SourceType string `json:"sourceType"`
	SourceID   int64  `json:"sourceId"`
	Outcome    string `json:"outcome"`
	SupplierID int64  `json:"supplierId"`
	Amount     string `json:"amount"`
	Note       string `json:"note"`
}

// ActivityRow is one "who did what" entry. Amount is a signed money delta
// (cash drawer, lockers), "" when the source has none.
type ActivityRow struct {
	When   string `json:"when"`
	User   string `json:"user"`
	Source string `json:"source"`
	Action string `json:"action"`
	Detail string `json:"detail"`
	Amount string `json:"amount"`
}

// ActivityPage is one page of Activity; Next is the cursor for the page after
// it ("" = that was the last).
type ActivityPage struct {
	Rows []ActivityRow `json:"rows"`
	Next string        `json:"next"`
}

// ActivityOption is one Who or Source choice.
type ActivityOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ActivityFilters are Activity's Who (staff, never the system user) and
// Source (the core trails + plugin contributors) choices.
type ActivityFilters struct {
	Users   []CashierOption  `json:"users"`
	Sources []ActivityOption `json:"sources"`
}
