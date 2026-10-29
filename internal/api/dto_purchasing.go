package api

// MoneySource is somewhere admin money can come from or go to: a locker (with
// its live balance) or a cashier's open till. Value is what requests send back
// ("locker:3", "till:5"); "" means outside the shop (card/online not tracked).
type MoneySource struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Group   string `json:"group"` // "Lockers" | "Tills"
	Kind    string `json:"kind"`  // locker kind (safe, bank, pocket, other, card) or "till"
	Balance string `json:"balance"`
}

// AdminSupplierRow is one supplier on the admin list. Owed < 0 = they owe us
// (returns worth more than was owed, or an advance). OldDebt is the still-unpaid
// pre-system opening, the only part of Owed an admin may correct.
type AdminSupplierRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Contact    string `json:"contact"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	CreditDays int    `json:"creditDays"`
	Owed       string `json:"owed"`
	OldDebt    string `json:"oldDebt"`
	Active     bool   `json:"active"`
}

type AdminSupplierList struct {
	Items    []AdminSupplierRow `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	// Owed / Credit sum every matching supplier (payables vs what suppliers owe us).
	Owed   string `json:"owed"`
	Credit string `json:"credit"`
}

// SupplierMoneyRow is one payment to, or refund from, a supplier.
type SupplierMoneyRow struct {
	Kind      string `json:"kind"` // payment | refund
	Date      string `json:"date"`
	Method    string `json:"method"`
	Reference string `json:"reference"`
	Note      string `json:"note"`
	By        string `json:"by"`
	Amount    string `json:"amount"`
}

type AdminSupplierDetail struct {
	AdminSupplierRow `tstype:",extends"`
	Invoices         []SupplierInvoice  `json:"invoices"` // open, oldest first
	Orders           []SupplierOrder    `json:"orders"`   // drafts not yet received
	Money            []SupplierMoneyRow `json:"money"`    // newest first
	Products         int                `json:"products"` // products defaulting to this supplier
}

// SupplierRequest creates or edits a supplier. OpeningBalance is read on create
// only; afterwards it changes through the opening adjust.
type SupplierRequest struct {
	Name           string `json:"name"`
	Contact        string `json:"contact"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	CreditDays     int    `json:"creditDays"`
	OpeningBalance string `json:"openingBalance"`
}

type PayAlloc struct {
	PurchaseID int64  `json:"purchaseId"`
	Amount     string `json:"amount"`
}

// AdminSupplierPayRequest pays a supplier from the back office. With no
// allocations, advance or oldDebt, Amount waterfalls (Mode "pay" = invoices
// first, "old" = old debt first); otherwise the split is exactly as given.
type AdminSupplierPayRequest struct {
	Amount      string     `json:"amount"`
	Mode        string     `json:"mode"`
	Allocations []PayAlloc `json:"allocations"`
	Advance     string     `json:"advance"`
	OldDebt     string     `json:"oldDebt"`
	Method      string     `json:"method"`
	Reference   string     `json:"reference"`
	Note        string     `json:"note"`
	Source      string       `json:"source"` // MoneySource value; "" = outside the shop (not for cash)
	Cheques     []ChequeLine `json:"cheques,omitempty"` // Method cheque: drawn on bank places, adding up to the payment
}

// AdminMoneyRequest is money a party hands back (supplier refund).
type AdminMoneyRequest struct {
	Amount    string `json:"amount"`
	Method    string `json:"method"`
	Reference string `json:"reference"`
	Note      string `json:"note"`
	Source    string `json:"source"`
}

type ReassignRequest struct {
	ToID int64 `json:"toId"`
}

type CountResult struct {
	Count int `json:"count"`
}

// PurchaseRow is one purchase order (draft) or received delivery (GRN).
type PurchaseRow struct {
	ID           int64  `json:"id"`
	SupplierID   int64  `json:"supplierId"`
	SupplierName string `json:"supplierName"`
	InvoiceNo    string `json:"invoiceNo"`
	Status       string `json:"status"` // draft | received | partial | paid (karots-pos purchase_status)
	Date         string `json:"date"`
	ExpectedDate string `json:"expectedDate"` // "" = none
	DueDate      string `json:"dueDate"`
	Total        string `json:"total"`
	Paid         string `json:"paid"`
	Credited     string `json:"credited"` // value returned against it
	Balance      string `json:"balance"`
	By           string `json:"by"`
}

type PurchaseList struct {
	Items    []PurchaseRow `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

// PurchaseLine is a line as stored, plus the product's current cost and price
// (the receive screen's margin guard). Ordered is "" for lines not on the order.
type PurchaseLine struct {
	StockLine `tstype:",extends"`
	Net       string `json:"net"` // line payable after its discount
	CurCost   string `json:"curCost"`
	CurSell   string `json:"curSell"`
	UnitAbbr  string `json:"unitAbbr"`
}

// PurchaseParty is the supplier block on a PO / GRN.
type PurchaseParty struct {
	Name    string `json:"name"`
	Contact string `json:"contact"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

type PurchaseDetail struct {
	PurchaseRow  `tstype:",extends"`
	Supplier     PurchaseParty  `json:"supplier"`
	Subtotal     string         `json:"subtotal"`
	Discount     string         `json:"discount"`      // resolved bill discount
	DiscountType string         `json:"discountType"`  // as entered
	DiscountVal  string         `json:"discountValue"` // as entered
	Notes        string         `json:"notes"`
	Lines        []PurchaseLine `json:"lines"`
}

// AdminPayNow is paying on delivery from the back office (MoneySource value).
type AdminPayNow struct {
	Amount  string       `json:"amount"` // "" / 0 = all on credit
	Method  string       `json:"method"`
	Source  string       `json:"source"`
	Cheques []ChequeLine `json:"cheques,omitempty"` // Method cheque
}

// PurchaseRequest is an order (draft), an edit of one, a walk-in delivery or
// receiving an order. Invoice / due / pay / keepRemainder apply to receiving.
type PurchaseRequest struct {
	SupplierID    int64       `json:"supplierId"`
	InvoiceNo     string      `json:"invoiceNo"`
	Notes         string      `json:"notes"`
	ExpectedDate  string      `json:"expectedDate"`
	DueDate       string      `json:"dueDate"`
	Discount      string      `json:"discount"`
	DiscountType  string      `json:"discountType"`
	Lines         []StockLine `json:"lines"`
	KeepRemainder bool        `json:"keepRemainder"`
	Pay           AdminPayNow `json:"pay"`
}

// ReorderRow is a product at or under its reorder level. Suggested is the
// demand-based order qty (90-day sales × 14-day lead − on hand), or 2 × reorder
// level − on hand with no sales history. Sold* are units sold in that window.
type ReorderRow struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	UnitAbbr     string `json:"unitAbbr"`
	OnHand       string `json:"onHand"`
	ReorderLevel int    `json:"reorderLevel"`
	Cost         string `json:"cost"`
	SupplierID   int64  `json:"supplierId"` // preferred; 0 = none
	SupplierName string `json:"supplierName"`
	Suggested    string `json:"suggested"`
	FromSales    bool   `json:"fromSales"` // Suggested came from sales history
	SoldWeek     string `json:"soldWeek"`
	SoldMonth    string `json:"soldMonth"`
	SoldLastYear string `json:"soldLastYear"` // the same 90 days a year ago
	Note         string `json:"note"`         // a plugin's note (e.g. an equivalent in stock)
	Covered      bool   `json:"covered"`      // a plugin says an equivalent covers it
	// Smart reorder levels: the mode, the learned numbers (nil = not run yet) and
	// whether a Fixed level is far from what sales suggest.
	Auto     bool             `json:"auto"`
	Calc     *ReorderCalcView `json:"calc,omitempty"`
	LooksOff bool             `json:"looksOff"`
}

// ReorderLevelRequest: Use "suggested" sets each Fixed product to what sales
// suggest (Use N / Use all); "auto" switches them to Auto instead.
type ReorderLevelRequest struct {
	IDs []int64 `json:"ids"`
	Use string  `json:"use"`
}

// ReorderAutoRequest switches every Fixed product of a category (with its
// sub-categories) to Auto; CategoryID 0 = all products.
type ReorderAutoRequest struct {
	CategoryID int64 `json:"categoryId"`
}

type ReorderAutoResult struct {
	Switched int `json:"switched"`
}

// ReorderList: Annotated = some plugin notes rows, so the page offers Hide
// covered items (?hideCovered=1); Hidden is how many covered rows that hid.
type ReorderList struct {
	Items     []ReorderRow `json:"items"`
	Total     int          `json:"total"`
	Page      int          `json:"page"`
	PageSize  int          `json:"pageSize"`
	Annotated bool         `json:"annotated"`
	Hidden    int          `json:"hidden"`
}

type ReorderLine struct {
	ProductID  int64  `json:"productId"`
	SupplierID int64  `json:"supplierId"`
	Quantity   string `json:"quantity"`
	CostPrice  string `json:"costPrice"`
}

// ReorderRequest builds one purchase order per supplier from the picked lines.
type ReorderRequest struct {
	Lines []ReorderLine `json:"lines"`
}

type IDsResult struct {
	IDs []int64 `json:"ids"`
}

// PurchaseReturnRow is a debit note: stock sent back to a supplier.
type PurchaseReturnRow struct {
	ID           int64  `json:"id"`
	SupplierID   int64  `json:"supplierId"`
	SupplierName string `json:"supplierName"`
	PurchaseID   int64  `json:"purchaseId"` // the delivery it went back from; 0 = an old free-form return
	InvoiceNo    string `json:"invoiceNo"`  // that delivery's invoice no
	Reference    string `json:"reference"`
	Reason       string `json:"reason"`
	Total        string `json:"total"`
	Date         string `json:"date"`
}

type PurchaseReturnList struct {
	Items    []PurchaseReturnRow `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

type PurchaseReturnLine struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Quantity  string `json:"quantity"`
	CostPrice string `json:"costPrice"`
	Subtotal  string `json:"subtotal"`
	BatchID   int64  `json:"batchId"` // 0 = oldest first
}

type PurchaseReturnDetail struct {
	PurchaseReturnRow `tstype:",extends"`
	Lines             []PurchaseReturnLine `json:"lines"`
}

type PurchaseReturnItem struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
	CostPrice string `json:"costPrice"` // ignored: the lot's cost is what goes back
	BatchID   int64  `json:"batchId"`   // the lot the delivery created
}

// PurchaseReturnRequest sends part of one received delivery back, at what those
// units really cost (after discounts and free units); the value credits that
// delivery's bill first, then the supplier's other open bills oldest first.
type PurchaseReturnRequest struct {
	SupplierID int64                `json:"supplierId"`
	PurchaseID int64                `json:"purchaseId"`
	Reference  string               `json:"reference"`
	Reason     string               `json:"reason"`
	Lines      []PurchaseReturnItem `json:"lines"`
}

// ReturnableLine is one line of a received delivery that can still go back:
// Left is what remains of the lot it created; Cost is that lot's real unit cost
// (after line + bill discounts and free units), ListCost what the bill said.
type ReturnableLine struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	UnitAbbr  string `json:"unitAbbr"`
	BatchID   int64  `json:"batchId"`
	Received  string `json:"received"` // paid + free
	Left      string `json:"left"`
	Cost      string `json:"cost"`
	ListCost  string `json:"listCost"`
	Expiry    string `json:"expiry"`
}

type ReturnableList struct {
	Lines []ReturnableLine `json:"lines"`
}
