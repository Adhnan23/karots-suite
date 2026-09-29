package api

// ---- Till: config ----

type TenderMethod struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TillConfig is everything the till needs from settings + the user's flags,
// loaded once when the till opens.
type TillConfig struct {
	Symbol             string         `json:"symbol"`
	DefaultSaleType    string         `json:"defaultSaleType"` // "retail" | "wholesale"
	AskToPrint         bool           `json:"askToPrint"`
	OpenCashDrawer     bool           `json:"openCashDrawer"`
	AllowUntrackedCash bool           `json:"allowUntrackedCash"`
	DefaultLockerID    int64          `json:"defaultLockerId"` // 0 = none
	AllowContinueLast  bool           `json:"allowContinueLast"`
	CanManageCredit    bool           `json:"canManageCredit"`
	CanHandleSuppliers bool           `json:"canHandleSuppliers"` // Suppliers + Expenses tabs
	CanSeeCost         bool           `json:"canSeeCost"`
	CanChangePin       bool           `json:"canChangePin"`
	IsManager          bool           `json:"isManager"`     // admin/manager: shows the Admin link
	TenderMethods      []TenderMethod `json:"tenderMethods"` // plugin tenders; core ones are fixed in the SPA
	LabelSize          string         `json:"labelSize"`     // the shop's default sticker, "WxH" mm
}

// ---- Till: catalog ----

type SaleSuggestion struct {
	DiscountType  string `json:"discountType"` // "fixed" | "percent"
	DiscountValue string `json:"discountValue"`
	Label         string `json:"label"`
	Prompt        string `json:"prompt"`
}

// TillProduct is one sellable card. Money and quantities are decimal strings.
type TillProduct struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	Barcode          string          `json:"barcode"`
	SellingPrice     string          `json:"sellingPrice"`
	WholesalePrice   string          `json:"wholesalePrice"`
	TaxRate          string          `json:"taxRate"`
	StockQty         string          `json:"stockQty"`
	UnitAbbr         string          `json:"unitAbbr"`
	UnitAllowDecimal bool            `json:"unitAllowDecimal"`
	IsService        bool            `json:"isService"`
	TrackSerial      bool            `json:"trackSerial"`
	WarrantyMonths   int             `json:"warrantyMonths"`
	Emoji            string          `json:"emoji"`
	Badges           []string        `json:"badges"`
	Suggestion       *SaleSuggestion `json:"suggestion,omitempty"`
}

type ProductPage struct {
	Items []TillProduct `json:"items"`
	Total int           `json:"total"`
}

type TillGroup struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Emoji       string `json:"emoji"`
	HasChildren bool   `json:"hasChildren"`
	ItemCount   int    `json:"itemCount"`
}

type GroupView struct {
	Group      TillGroup     `json:"group"`
	Breadcrumb []TillGroup   `json:"breadcrumb"`
	Children   []TillGroup   `json:"children"`
	Products   []TillProduct `json:"products"`
}

// Lot is one live stock batch. ExpiryDate/ReceivedAt are YYYY-MM-DD ("" = none).
type Lot struct {
	BatchID      int64  `json:"batchId"`
	BatchNo      string `json:"batchNo"`
	ExpiryDate   string `json:"expiryDate"`
	QtyRemaining string `json:"qtyRemaining"`
	Price        string `json:"price"`
	ReceivedAt   string `json:"receivedAt"`
}

type DetailRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ProductDetail feeds the ⓘ popup. Cost fields are empty unless ShowCost.
type ProductDetail struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	Category     string      `json:"category"`
	Barcode      string      `json:"barcode"`
	Unit         string      `json:"unit"`
	SellingPrice string      `json:"sellingPrice"`
	StockQty     string      `json:"stockQty"`
	Warranty     string      `json:"warranty"`
	ShowCost     bool        `json:"showCost"`
	CostPrice    string      `json:"costPrice"`
	MarginPct    string      `json:"marginPct"`
	Rows         []DetailRow `json:"rows"`
}

type Unit struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	AllowDecimal bool   `json:"allowDecimal"`
}

// ---- Till: customers ----

// TillCustomer: Outstanding > 0 means they owe the shop; < 0 is an advance the
// shop holds for them.
type TillCustomer struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit string `json:"creditLimit"`
	Outstanding string `json:"outstanding"`
}

type CustomerCreateRequest struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit string `json:"creditLimit"`
}

type CustomerCreated struct {
	Customer TillCustomer `json:"customer"`
	Existed  bool         `json:"existed"` // phone matched an existing customer; that one is returned
}

type CreditLimitRequest struct {
	CreditLimit string `json:"creditLimit"`
}

// ---- Till: quick item / write-off ----

type QuickItemRequest struct {
	Name    string `json:"name"`
	Price   string `json:"price"`
	Qty     string `json:"qty"`
	Barcode string `json:"barcode"`
	UnitID  int64  `json:"unitId"`
}

type GeneratedBarcode struct {
	Barcode string `json:"barcode"`
}

// WriteOffRequest removes a lot from stock as damage at the till (the expired-lot
// prompt's "Write off" button).
type WriteOffRequest struct {
	ProductID int64  `json:"productId"`
	BatchID   int64  `json:"batchId"`
	Quantity  string `json:"quantity"`
	Note      string `json:"note"`
	Reason    string `json:"reason"` // damage (default) | own_use | staff — decides the P&L line
}
