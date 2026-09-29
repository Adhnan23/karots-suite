package api

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
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	NameLocal    string `json:"nameLocal"`
	Barcode      string `json:"barcode"`
	CategoryID   int64  `json:"categoryId"`
	CategoryName string `json:"categoryName"`
	UnitAbbr     string `json:"unitAbbr"`
	CostPrice    string `json:"costPrice"`
	SellingPrice string `json:"sellingPrice"`
	Wholesale    string `json:"wholesalePrice"`
	StockQty     string `json:"stockQty"`
	ReorderLevel int    `json:"reorderLevel"`
	Low          bool   `json:"low"` // on hand at or below the reorder level
	IsActive     bool   `json:"isActive"`
	IsService    bool   `json:"isService"`
	NeedsReview  bool   `json:"needsReview"`
	TrackSerial  bool   `json:"trackSerial"`
	HasExpiry    bool   `json:"hasExpiry"`
	SupplierName string `json:"supplierName"`
	CreatedBy    string `json:"createdBy"` // who quick-added it (review queue)
	CreatedAt    string `json:"createdAt"`
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
	UnitID         int64  `json:"unitId"`
	TaxRate        string `json:"taxRate"`
	WarrantyMonths int    `json:"warrantyMonths"`
	SupplierID     int64  `json:"supplierId"`
}

// ProductRequest creates or edits a product. Money and tax are decimal strings;
// SupplierID 0 = none. RepriceStock makes lots on the shelf follow a new selling
// price. DupSourceName is set when saving a Duplicate: the copy must be renamed.
type ProductRequest struct {
	Name           string `json:"name"`
	NameLocal      string `json:"nameLocal"`
	Barcode        string `json:"barcode"`
	CategoryID     int64  `json:"categoryId"`
	UnitID         int64  `json:"unitId"`
	CostPrice      string `json:"costPrice"`
	SellingPrice   string `json:"sellingPrice"`
	WholesalePrice string `json:"wholesalePrice"`
	TaxRate        string `json:"taxRate"`
	ReorderLevel   int    `json:"reorderLevel"`
	TrackSerial    bool   `json:"trackSerial"`
	WarrantyMonths int    `json:"warrantyMonths"`
	SupplierID     int64  `json:"supplierId"`
	RepriceStock   bool   `json:"repriceStock"`
	DupSourceName  string `json:"dupSourceName"`
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
