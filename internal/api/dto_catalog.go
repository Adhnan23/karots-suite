package api

import "encoding/json"

// ---- Admin: catalog (units, categories, products, services, groups) ----

type UnitRequest struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	AllowDecimal bool   `json:"allowDecimal"`
}

// CategoryNode is one row of the depth-first category tree. ParentID 0 = top level;
// Path holds ancestor ids, root first (not self).
type CategoryNode struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ParentID     int64   `json:"parentId"`
	Depth        int     `json:"depth"`
	ChildCount   int     `json:"childCount"`
	Path         []int64 `json:"path"`
	ProductCount int     `json:"productCount"` // products filed directly here
}

// CategoryRequest creates or edits a category. ParentID 0 = top level.
type CategoryRequest struct {
	Name     string `json:"name"`
	ParentID int64  `json:"parentId"`
}

// CategoryQuickResult answers the pickers' inline create (find-or-create under a parent).
type CategoryQuickResult struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Depth int    `json:"depth"`
}

// AdminProductRow is one line of the admin product list.
type AdminProductRow struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	NameLocal    string   `json:"nameLocal"`
	Barcode      string   `json:"barcode"`  // the main barcode (labels print it)
	Barcodes     []string `json:"barcodes"` // extra barcodes, oldest first
	CategoryID   int64    `json:"categoryId"`
	CategoryName string   `json:"categoryName"`
	UnitAbbr     string   `json:"unitAbbr"`
	CostPrice    string   `json:"costPrice"`
	SellingPrice string   `json:"sellingPrice"`
	Wholesale    string   `json:"wholesalePrice"`
	StockQty     string   `json:"stockQty"`
	ReorderLevel int      `json:"reorderLevel"`
	Low          bool     `json:"low"` // on hand at or below the reorder level
	IsActive     bool     `json:"isActive"`
	IsService    bool     `json:"isService"`
	NeedsReview  bool     `json:"needsReview"`
	TrackSerial  bool     `json:"trackSerial"`
	HasExpiry    bool     `json:"hasExpiry"`
	SupplierName string   `json:"supplierName"`
	CreatedBy    string   `json:"createdBy"` // who quick-added it (review queue)
	TillPack     string   `json:"tillPack"`  // pack sizes set at the till, to check (review queue)
	CreatedAt    string   `json:"createdAt"`
	Meta         string   `json:"meta"` // a plugin's short line under the name ("Grade: B · Waterproof")
}

// ProductBarcode is one of a product's codes for the form's Barcodes block: the
// main one first, then extras. Dates are ISO 8601; "" = never scanned.
type ProductBarcode struct {
	Barcode       string `json:"barcode"`
	Main          bool   `json:"main"`
	Source        string `json:"source"` // intake | receive | form | merge | import ("" for the main one)
	AddedAt       string `json:"addedAt"`
	LastScannedAt string `json:"lastScannedAt"`
}

// AdminPack is one way to sell / buy a product by more than one unit (Card ×12).
// Prices "0.00" mean: follows the unit price (selling) / the selling price (wholesale).
type AdminPack struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Size           string   `json:"size"`
	SellingPrice   string   `json:"sellingPrice"`
	WholesalePrice string   `json:"wholesalePrice"`
	SetAtTill      bool     `json:"setAtTill"`
	Barcodes       []string `json:"barcodes"`
}

// PackRequest creates / edits a pack; Barcode adds one code to it ("" = none).
type PackRequest struct {
	Name           string `json:"name"`
	Size           string `json:"size"`
	SellingPrice   string `json:"sellingPrice"`
	WholesalePrice string `json:"wholesalePrice"`
	Barcode        string `json:"barcode"`
}

// BarcodeRequest adds an extra barcode; Source says where it was added.
type BarcodeRequest struct {
	Barcode string `json:"barcode"`
	Source  string `json:"source"`
}

// MergePreview is what merging one product into another would move (the confirm
// screen). Refusal, when set, is why it can't run. Prices and stock are decimal
// strings; PriceClash = the prices differ and some of its stock follows its own.
type MergePreview struct {
	FromName   string `json:"fromName"`
	IntoName   string `json:"intoName"`
	Barcodes   int    `json:"barcodes"`
	Stock      string `json:"stock"`
	Lots       int    `json:"lots"`
	Sales      int    `json:"sales"`
	Purchases  int    `json:"purchases"`
	PriceClash bool   `json:"priceClash"`
	FromPrice  string `json:"fromPrice"`
	IntoPrice  string `json:"intoPrice"`
	Refusal    string `json:"refusal"`
}

// MergeRequest merges; LotPrice "keep" keeps the duplicate's stock at its own price,
// "kept" lets it follow the kept product's.
type MergeRequest struct {
	LotPrice string `json:"lotPrice"`
}

// ProductList is a page of the admin product list.
type ProductList struct {
	Items    []AdminProductRow `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

// AdminProduct is a product for the edit form.
type AdminProduct struct {
	AdminProductRow `tstype:",extends"`
	UnitID          int64  `json:"unitId"`
	TaxRate         string `json:"taxRate"`
	WarrantyMonths  int    `json:"warrantyMonths"`
	SupplierID      int64  `json:"supplierId"`
}

// SheetExtra is a plugin's column group that Products → Sheets can include.
type SheetExtra struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ProductRequest creates or edits a product. Money and tax are decimal strings;
// SupplierID 0 = none. RepriceStock makes lots on the shelf follow a new selling
// price. DupSourceName is set when saving a Duplicate: the copy must be renamed.
// Extra holds each plugin's own data by key (opaque to core; see plugin.ProductExtras).
type ProductRequest struct {
	Name           string                     `json:"name"`
	NameLocal      string                     `json:"nameLocal"`
	Barcode        string                     `json:"barcode"`
	CategoryID     int64                      `json:"categoryId"`
	UnitID         int64                      `json:"unitId"`
	CostPrice      string                     `json:"costPrice"`
	SellingPrice   string                     `json:"sellingPrice"`
	WholesalePrice string                     `json:"wholesalePrice"`
	TaxRate        string                     `json:"taxRate"`
	ReorderLevel   int                        `json:"reorderLevel"`
	TrackSerial    bool                       `json:"trackSerial"`
	WarrantyMonths int                        `json:"warrantyMonths"`
	SupplierID     int64                      `json:"supplierId"`
	RepriceStock   bool                       `json:"repriceStock"`
	DupSourceName  string                     `json:"dupSourceName"`
	Extra          map[string]json.RawMessage `json:"extra,omitempty" tstype:"{ [key: string]: unknown }"`
}

// ProductSaved answers a product save; Message says what else happened (a
// finished review item gets its past sale lines' cost corrected).
type ProductSaved struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

// Option is an id + name pair for pickers (suppliers, units).
type Option struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Count struct {
	Count int `json:"count"`
}

// ServiceRow is one service product with its recipe cost (karots-pos Services).
// Costs are "" when it has no recipe yet.
type ServiceRow struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CategoryName string `json:"categoryName"`
	UnitAbbr     string `json:"unitAbbr"`
	SellingPrice string `json:"sellingPrice"`
	StockCost    string `json:"stockCost"`
	OtherCost    string `json:"otherCost"`
	TrueCost     string `json:"trueCost"`
	Margin       string `json:"margin"`
	Ingredients  int    `json:"ingredients"`
	IsActive     bool   `json:"isActive"`
}

// ServiceRequest creates a non-stocked product. Only what a service carries.
type ServiceRequest struct {
	Name         string `json:"name"`
	CategoryID   int64  `json:"categoryId"`
	UnitID       int64  `json:"unitId"`
	SellingPrice string `json:"sellingPrice"`
}

// RecipeComponent is one stock ingredient. Mode "per_unit" = Amount used per
// unit sold; "yield" = one unit of the ingredient makes Amount sales. Whole
// rounds usage up (a sheet of paper can't be part used).
type RecipeComponent struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	UnitAbbr  string `json:"unitAbbr"`
	CostPrice string `json:"costPrice"`
	Amount    string `json:"amount"`
	Mode      string `json:"mode"`
	Whole     bool   `json:"whole"`
}

// RecipeCost is a non-stock cost per unit (electricity, a fee): an estimate for
// pricing, never charged to profit.
type RecipeCost struct {
	Label  string `json:"label"`
	Amount string `json:"amount"`
}

type Recipe struct {
	ProductID  int64             `json:"productId"`
	Name       string            `json:"name"`
	UnitAbbr   string            `json:"unitAbbr"`
	Selling    string            `json:"sellingPrice"`
	Components []RecipeComponent `json:"components"`
	Costs      []RecipeCost      `json:"costs"`
}

type RecipeRequest struct {
	Components []RecipeComponent `json:"components"`
	Costs      []RecipeCost      `json:"costs"`
}

// GroupNode is one till menu card group, depth-first. ParentID 0 = top level.
type GroupNode struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Emoji      string `json:"emoji"`
	ParentID   int64  `json:"parentId"`
	Depth      int    `json:"depth"`
	IsActive   bool   `json:"isActive"`
	ItemCount  int    `json:"itemCount"`
	ChildCount int    `json:"childCount"`
}

// GroupRequest creates or edits a group. ParentID is only read on create
// (karots-pos never moves a group to another parent).
type GroupRequest struct {
	Name     string `json:"name"`
	Emoji    string `json:"emoji"`
	ParentID int64  `json:"parentId"`
}

// GroupItem is a product on a group card; the emoji lives on the link.
type GroupItem struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Barcode      string `json:"barcode"`
	Emoji        string `json:"emoji"`
	SellingPrice string `json:"sellingPrice"`
	StockQty     string `json:"stockQty"`
	UnitAbbr     string `json:"unitAbbr"`
	IsService    bool   `json:"isService"`
}

type GroupItemRequest struct {
	ProductID int64  `json:"productId"`
	Emoji     string `json:"emoji"`
}

// LooseVersionRequest makes a packed product's loose version in one go: the
// product (category, supplier, tax, reorder, warranty, local name copied from the
// source), the conversion "1 source unit = Ratio of these", and — when
// ConvertNow (source units) is above zero — the first run. Extra is each
// plugin's data for the new product, as on the product form.
type LooseVersionRequest struct {
	Name         string                     `json:"name"`
	UnitID       int64                      `json:"unitId"`
	Ratio        string                     `json:"ratio"`
	SellingPrice string                     `json:"sellingPrice"`
	Barcode      string                     `json:"barcode"`
	ConvertNow   string                     `json:"convertNow"`
	Extra        map[string]json.RawMessage `json:"extra,omitempty" tstype:"{ [key: string]: unknown }"`
}

// LooseVersionResult names what was made.
type LooseVersionResult struct {
	ProductID    int64 `json:"productId"`
	ConversionID int64 `json:"conversionId"`
}
