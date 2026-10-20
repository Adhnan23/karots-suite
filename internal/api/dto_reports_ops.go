package api

// ---- Reports (M5), part 2: stock, losses, activity, dashboard ----

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

// ReadinessGroup is one plugin's readiness section (e.g. a plugin's float).
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
	ProductID int64  `json:"productId"`
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
	BatchID    int64  `json:"batchId"` // replacement: the batch it joins (0 = a new batch)
	Expiry     string `json:"expiry"`  // replacement in a new batch: its expiry, YYYY-MM-DD or ""
	// Replacement: what the supplier asked on top, per unit (it costs more now;
	// 0 = free) — owed to SupplierID and added to the batch cost — and the new
	// batch's selling price ("" = unchanged; a new price also goes on the product).
	ExtraPerUnit string `json:"extraPerUnit"`
	SellingPrice string `json:"sellingPrice"`
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

// DashboardDay is one shop day's takings, counted as the P&L counts them.
type DashboardDay struct {
	Count     int    `json:"count"`
	Revenue   string `json:"revenue"`
	Profit    string `json:"profit"`
	Margin    string `json:"margin"`
	AvgBasket string `json:"avgBasket"`
}

// DashboardAttention is what needs someone today (each 0 = nothing).
type DashboardAttention struct {
	LowStock     int    `json:"lowStock"`
	Expired      int    `json:"expired"`
	Expiring     int    `json:"expiring"` // within 30 days
	Review       int    `json:"review"`   // quick-added items needing details
	OverLimit    int    `json:"overLimit"`
	OverLimitSum string `json:"overLimitSum"`
	// BackupCopies names the switched-on backup copies with no copy in 24 h.
	BackupCopies []string `json:"backupCopies"`
	// StaleTills are tills left open from an earlier day.
	StaleTills []StaleTill `json:"staleTills"`
	// NoTillSales are today's sales that took cash while their cashier had no till open.
	NoTillSales []NoTillSale `json:"noTillSales"`
}

type StaleTill struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"userId"`
	Cashier  string `json:"cashier"`
	OpenedAt string `json:"openedAt"`
}

type NoTillSale struct {
	ID        int64  `json:"id"`
	ReceiptNo string `json:"receiptNo"`
	Cashier   string `json:"cashier"`
	At        string `json:"at"`
	Cash      string `json:"cash"` // cash kept: cash paid − change
}

// Dashboard is the admin home (karots-pos Dashboard): today against
// yesterday, the last 14 days for the sparklines, money in the open tills
// and owed to the shop, today's payment mix, what needs attention, and the
// latest sales.
type Dashboard struct {
	Today     DashboardDay       `json:"today"`
	Yesterday DashboardDay       `json:"yesterday"`
	Spark     []FinancePoint     `json:"spark"`
	TillCash  string             `json:"tillCash"`
	OpenTills int                `json:"openTills"`
	Owed      string             `json:"owed"`
	Owing     int                `json:"owing"`
	Paid      []FinanceSlice     `json:"paid"`
	Attention DashboardAttention `json:"attention"`
	Recent    []AdminSaleRow     `json:"recent"`
}

// DashboardCharts is the chart range (karots-pos dashboard carousel, 7 / 14 /
// 30 / 90 days): revenue per period with the period before laid beside it
// (same length, Prev[i] is the matching earlier period), and the top sellers.
type DashboardCharts struct {
	Days  int             `json:"days"`
	Grain string          `json:"grain"`
	Cur   []FinancePoint  `json:"cur"`
	Prev  []FinancePoint  `json:"prev"`
	Top   []TopProductRow `json:"top"`
}

// ClaimLot is an expired batch still on the shelf, to settle with its supplier.
type ClaimLot struct {
	LotID     int64  `json:"lotId"`
	ProductID int64  `json:"productId"`
	Product   string `json:"product"`
	Unit      string `json:"unit"`
	Qty       string `json:"qty"`
	Expiry    string `json:"expiry"`
	Cost      string `json:"cost"`
}

// SupplierClaims is what to settle with a supplier when their delivery comes:
// expired batches on the shelf (write off, then settle) and write-offs still
// waiting on them.
type SupplierClaims struct {
	Expired []ClaimLot `json:"expired"`
	Waiting []LossRow  `json:"waiting"`
}
