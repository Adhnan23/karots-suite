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
// the lot a reduction comes off (0 = oldest first).
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
}

type ProductLots struct {
	ProductID    int64    `json:"productId"`
	Name         string   `json:"name"`
	UnitAbbr     string   `json:"unitAbbr"`
	SellingPrice string   `json:"sellingPrice"`
	StockQty     string   `json:"stockQty"`
	Lots         []LotRow `json:"lots"`
}

// LotPriceRequest re-prices one lot; "" = follow the product again.
type LotPriceRequest struct {
	SellingPrice string `json:"sellingPrice"`
}
