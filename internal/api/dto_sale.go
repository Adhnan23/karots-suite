package api

type ServiceComponent struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
}

// SaleItemRequest never carries a price for stocked goods: the server prices from
// the catalogue, or from BatchID when the cashier picked a lot. PriceOverride is
// honoured only for is_service products.
type SaleItemRequest struct {
	ProductID     int64              `json:"productId"`
	Quantity      string             `json:"quantity"`
	Discount      string             `json:"discount"`
	DiscountType  string             `json:"discountType"` // "fixed" (per unit) | "percent"
	PriceOverride string             `json:"priceOverride"`
	BatchID       int64              `json:"batchId"`
	AllowOversell bool               `json:"allowOversell"`
	Serials       []string           `json:"serials"`
	Description   string             `json:"description"`
	Components    []ServiceComponent `json:"components"`
	Group         *SaleLineGroup     `json:"group,omitempty"` // lines added together; receipts print them as one block
	// Plugin is the cart line's opaque plugin data by plugin key (CartLine.plugin);
	// each plugin's sale-line hook reads its own key inside the sale's tx.
	Plugin map[string]any `json:"plugin,omitempty"`
}

// SaleLineGroup: no numbers the groups within the sale, sets is how many sets.
type SaleLineGroup struct {
	No    int    `json:"no"`
	Label string `json:"label"`
	Sets  string `json:"sets"`
}

type PaymentRequest struct {
	Method    string            `json:"method"` // cash | card | online | credit | <plugin tender>
	Amount    string            `json:"amount"`
	Reference string            `json:"reference"`
	AccountID int64             `json:"accountId"`      // card/bank locker for card/online; 0 = none
	Meta      map[string]string `json:"meta,omitempty"` // opaque plugin-tender data
}

type SaleRequest struct {
	SaleType          string            `json:"saleType"` // retail | wholesale
	CustomerID        *int64            `json:"customerId,omitempty"`
	Discount          string            `json:"discount"`
	DiscountType      string            `json:"discountType"`
	AllowOverLimit    bool              `json:"allowOverLimit"`
	DeferDrawerKick   bool              `json:"deferDrawerKick"`
	ChangeGiven       string            `json:"changeGiven"` // "" = full computed change
	OnAccountRounding bool              `json:"onAccountRounding"`
	Items             []SaleItemRequest `json:"items"`
	Payments          []PaymentRequest  `json:"payments"`
}

type SaleResult struct {
	ID                   int64    `json:"id"`
	ReceiptNo            string   `json:"receiptNo"`
	Total                string   `json:"total"`
	PaidAmount           string   `json:"paidAmount"`
	ChangeGiven          string   `json:"changeGiven"`
	RoundingToAccount    string   `json:"roundingToAccount"`
	CustomerName         string   `json:"customerName"`
	CustomerBalanceAfter *string  `json:"customerBalanceAfter,omitempty"`
	Methods              []string `json:"methods"` // distinct tender methods, for the complete overlay
}

// HeldSale.Cart is an opaque JSON document owned by the SPA (see
// web/src/features/till/cart/held.ts); the server only checks it is valid JSON.
type HeldSale struct {
	ID         int64  `json:"id"`
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID *int64 `json:"customerId,omitempty"`
	Discount   string `json:"discount"`
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
	CreatedAt  string `json:"createdAt"`
}

type HoldRequest struct {
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID *int64 `json:"customerId,omitempty"`
	Discount   string `json:"discount"`
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
}
