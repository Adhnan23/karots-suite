package api

// ---- Admin: stock (movements, adjust, lots, batches, stock-take, intake) ----

// MovementRow is one line of the stock audit trail. Quantity is signed (+in,
// −out); Cost is always the positive worth moved.
type MovementRow struct {
	ID          int64  `json:"id"`
	ProductID   int64  `json:"productId"`
	ProductName string `json:"productName"`
	Type        string `json:"type"`
	Quantity    string `json:"quantity"`
	Cost        string `json:"cost"`
	Note        string `json:"note"`
	UserName    string `json:"userName"`
	CreatedAt   string `json:"createdAt"`
}

type MovementList struct {
	Items    []MovementRow `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// AdjustRequest sets a product's on-hand to an absolute count. BatchID names
// the lot the change applies to: a reduction comes off it (0 = oldest first),
// an increase tops it up (0 = a new adjustment lot).
type AdjustRequest struct {
	ProductID   int64  `json:"productId"`
	NewQuantity string `json:"newQuantity"`
	Note        string `json:"note"`
	BatchID     int64  `json:"batchId"`
}

// LotRow is one live lot. SellingPrice "" = follows the product's price.
type LotRow struct {
	ID           int64  `json:"id"`
	BatchNo      string `json:"batchNo"`
	ExpiryDate   string `json:"expiryDate"` // YYYY-MM-DD or ""
	QtyReceived  string `json:"qtyReceived"`
	QtyRemaining string `json:"qtyRemaining"`
	CostPrice    string `json:"costPrice"`
	SellingPrice string `json:"sellingPrice"`
	Source       string `json:"source"`
	CreatedAt    string `json:"createdAt"`
	FromDelivery bool   `json:"fromDelivery"` // came on a purchase (returned against it); false = old / opening / adjusted stock
}

type ProductLots struct {
	ProductID    int64    `json:"productId"`
	Name         string   `json:"name"`
	UnitAbbr     string   `json:"unitAbbr"`
	SellingPrice string   `json:"sellingPrice"`
	StockQty     string   `json:"stockQty"`
	Lots         []LotRow `json:"lots"`
}

// AddBatchRequest opens a batch by hand (Batches → + Add batch). Blank cost =
// the product's, blank price = follows the product, blank expiry = none.
type AddBatchRequest struct {
	Qty          string `json:"qty"`
	Cost         string `json:"cost"`
	SellingPrice string `json:"sellingPrice"`
	Expiry       string `json:"expiry"` // YYYY-MM-DD or ""
}

// MergeBatchesRequest folds batches into the oldest of them with the values the
// prompt shows: blank cost = weighted average, blank expiry = none, blank
// price = follows the product.
type MergeBatchesRequest struct {
	BatchIDs     []int64 `json:"batchIds"`
	Cost         string  `json:"cost"`
	Expiry       string  `json:"expiry"`
	SellingPrice string  `json:"sellingPrice"`
}

// LotPriceRequest re-prices one lot; "" = follow the product again.
type LotPriceRequest struct {
	SellingPrice string `json:"sellingPrice"`
}

// BatchGroup is one product's live lots on the Batches page.
type BatchGroup struct {
	ProductID    int64    `json:"productId"`
	Name         string   `json:"name"`
	UnitAbbr     string   `json:"unitAbbr"`
	ProductPrice string   `json:"productPrice"`
	OnHand       string   `json:"onHand"`
	Value        string   `json:"value"`
	Expired      int      `json:"expired"`
	ExpiringSoon int      `json:"expiringSoon"` // within 30 days
	Lots         []LotRow `json:"lots"`
}

// BatchesView is every live lot grouped by product, expired first, with shop-wide
// expiry totals for the alert banners.
type BatchesView struct {
	TotalValue    string       `json:"totalValue"`
	Expired       int          `json:"expired"`
	ExpiredValue  string       `json:"expiredValue"`
	ExpiringSoon  int          `json:"expiringSoon"`
	ExpiringValue string       `json:"expiringValue"`
	NoExpiry      int          `json:"noExpiry"`
	Groups        []BatchGroup `json:"groups"`
}

// ConsumeRequest takes stock out without a sale: reason damage | own_use | staff.
// BatchID names the lot (0 = oldest first).
type ConsumeRequest struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
	Reason    string `json:"reason"`
	Note      string `json:"note"`
	BatchID   int64  `json:"batchId"`
}

type WriteOffResult struct {
	Lots int `json:"lots"`
}

// StockTakeRow is one product on the count screen. MultiPrice = its live lots
// carry different prices, so a count here can't say which lot a change is in.
type StockTakeRow struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Barcode          string `json:"barcode"`
	CategoryName     string `json:"categoryName"`
	UnitAbbr         string `json:"unitAbbr"`
	UnitAllowDecimal bool   `json:"unitAllowDecimal"`
	StockQty         string `json:"stockQty"`
	CostPrice        string `json:"costPrice"`
	SellingPrice     string `json:"sellingPrice"`
	WholesalePrice   string `json:"wholesalePrice"`
	MultiPrice       bool   `json:"multiPrice"`
}

type StockTakeList struct {
	Items    []StockTakeRow `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

// CountLine is one row of a count; blank fields leave that value alone.
type CountLine struct {
	ProductID int64  `json:"productId"`
	Counted   string `json:"counted"`
	Cost      string `json:"cost"`
	Selling   string `json:"selling"`
	Wholesale string `json:"wholesale"`
}

type StockTakeRequest struct {
	Lines []CountLine `json:"lines"`
}

type StockTakeResult struct {
	Saved int `json:"saved"`
}

type ImportRowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// ImportSummary answers any spreadsheet import.
type ImportSummary struct {
	Updated   int              `json:"updated"`
	Unchanged int              `json:"unchanged"`
	Skipped   int              `json:"skipped"`
	Errors    []ImportRowError `json:"errors"`
	Notes     []string         `json:"notes"`
}

// IntakeCreateRequest is a new product plus its opening count.
type IntakeCreateRequest struct {
	Product  ProductRequest `json:"product"`
	Quantity string         `json:"quantity"`
	Expiry   string         `json:"expiry"` // YYYY-MM-DD or ""
}

// IntakeRestockRequest adds Quantity to a product; blank prices stay as they are.
type IntakeRestockRequest struct {
	ProductID      int64  `json:"productId"`
	Quantity       string `json:"quantity"`
	CostPrice      string `json:"costPrice"`
	SellingPrice   string `json:"sellingPrice"`
	WholesalePrice string `json:"wholesalePrice"`
	Scanned        string `json:"scanned"` // the code scanned to open it, if any (stamps its last scan)
	BatchID        int64  `json:"batchId"` // the batch to top up; 0 = a new batch
	Expiry         string `json:"expiry"`  // a new batch's expiry, YYYY-MM-DD or ""
}

// IntakeUndoRequest backs out one session row; Kind "create" also disables the product.
type IntakeUndoRequest struct {
	ProductID int64  `json:"productId"`
	Qty       string `json:"qty"`
	Kind      string `json:"kind"`
	BatchID   int64  `json:"batchId"` // the batch the row went into (0 = oldest first)
}

// IntakeItem is one row of the intake session list (Qty = what this row added).
type IntakeItem struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Barcode      string `json:"barcode"`
	SellingPrice string `json:"sellingPrice"`
	Qty          string `json:"qty"`
	Kind         string `json:"kind"` // create | restock
	StockQty     string `json:"stockQty"`
	UnitAbbr     string `json:"unitAbbr"`
	BatchID      int64  `json:"batchId"` // the batch the units went into
}

// AdminConversion is a pack-breaking recipe: 1 From unit makes Ratio To units.
type AdminConversion struct {
	ID            int64  `json:"id"`
	FromProductID int64  `json:"fromProductId"`
	ToProductID   int64  `json:"toProductId"`
	FromName      string `json:"fromName"`
	ToName        string `json:"toName"`
	FromUnit      string `json:"fromUnit"`
	ToUnit        string `json:"toUnit"`
	Ratio         string `json:"ratio"`
	FromStock     string `json:"fromStock"`
	ToStock       string `json:"toStock"`
	Note          string `json:"note"`
	IsActive      bool   `json:"isActive"`
}

// ConversionRequest creates (products + ratio) or edits (ratio + note) a conversion.
type ConversionRequest struct {
	FromProductID int64  `json:"fromProductId"`
	ToProductID   int64  `json:"toProductId"`
	Ratio         string `json:"ratio"`
	Note          string `json:"note"`
}

type ConversionRunRow struct {
	ID        int64  `json:"id"`
	FromName  string `json:"fromName"`
	ToName    string `json:"toName"`
	FromQty   string `json:"fromQty"`
	ToQty     string `json:"toQty"`
	FromUnit  string `json:"fromUnit"`
	ToUnit    string `json:"toUnit"`
	UserName  string `json:"userName"`
	CreatedAt string `json:"createdAt"`
}

type ConversionRunList struct {
	Items    []ConversionRunRow `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}
