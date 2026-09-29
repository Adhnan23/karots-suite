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
