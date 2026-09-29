package api

// ---- Till counter operations (M1b): shared ----

// CounterMoneyRequest is the cash side of a counter action (expense, supplier
// payment…): where the cash comes from. LockerID 0 = the cashier's own drawer.
type CounterMoneyRequest struct {
	LockerID        int64 `json:"lockerId"`
	DeferDrawerKick bool  `json:"deferDrawerKick"` // auto-print folds the pulse into the slip job
}

// ---- Expenses ----

type ServiceOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CounterExpense struct {
	ID          int64  `json:"id"`
	Category    string `json:"category"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	ServiceName string `json:"serviceName"`
	PaidByName  string `json:"paidByName"`
	CreatedAt   string `json:"createdAt"`
}

// ExpensesPage is the Expenses tab: category suggestions, services a cost can be
// tagged to, and today's expenses.
type ExpensesPage struct {
	Categories []string         `json:"categories"`
	Services   []ServiceOption  `json:"services"`
	Today      []CounterExpense `json:"today"`
}

type ExpenseRequest struct {
	CounterMoneyRequest `tstype:",extends"`
	Category            string `json:"category"`
	Amount              string `json:"amount"`
	Description         string `json:"description"`
	ServiceProductID    int64  `json:"serviceProductId"`
}

// ---- Convert ----

// ConversionRow is one "break a pack" recipe: 1 From = Ratio × To.
type ConversionRow struct {
	ID        int64  `json:"id"`
	FromName  string `json:"fromName"`
	ToName    string `json:"toName"`
	FromUnit  string `json:"fromUnit"`
	ToUnit    string `json:"toUnit"`
	Ratio     string `json:"ratio"`
	FromStock string `json:"fromStock"`
	ToStock   string `json:"toStock"`
	Note      string `json:"note"`
}

type RunConversionRequest struct {
	Quantity string `json:"quantity"` // source units to convert
}

// ---- Labels ----

// LabelRequest prints Count copies of one sticker. ProductID fills a blank Code
// from the product (its barcode, else SKU<id>); Size "" = the shop default.
type LabelRequest struct {
	ProductID int64  `json:"productId"`
	Code      string `json:"code"`
	Format    string `json:"format"` // CODE128 (default) | EAN13 | EAN8 | UPC | CODE39
	Top       string `json:"top"`
	Bottom    string `json:"bottom"`
	Count     int    `json:"count"`
	Size      string `json:"size"` // "WxH" mm
}

// LabelLayout is where the printer puts everything on one label (dots, 8/mm,
// top-left origin) — the preview draws exactly this.
type LabelLayout struct {
	WidthMM    int          `json:"widthMm"`
	HeightMM   int          `json:"heightMm"`
	WidthDots  int          `json:"widthDots"`
	HeightDots int          `json:"heightDots"`
	Texts      []LabelText  `json:"texts"`
	Barcode    LabelBarcode `json:"barcode"`
}

type LabelText struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	CharW  int    `json:"charW"`
	GlyphH int    `json:"glyphH"`
	Value  string `json:"value"`
}

type LabelBarcode struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Height    int    `json:"height"`
	Narrow    int    `json:"narrow"`
	Format    string `json:"format"` // as requested ("" = CODE128)
	Code      string `json:"code"`
	ReadableH int    `json:"readableH"`
}

type AssignBarcodeRequest struct {
	Barcode string `json:"barcode"`
}

// ---- Sales lookup (Returns, Receipts) ----

type SaleSummary struct {
	ID           int64  `json:"id"`
	ReceiptNo    string `json:"receiptNo"`
	CreatedAt    string `json:"createdAt"`
	CustomerName string `json:"customerName"`
	CashierName  string `json:"cashierName"`
	Total        string `json:"total"`
	Status       string `json:"status"` // completed | partially_returned | returned | …
}

type SaleLine struct {
	ID          int64  `json:"id"` // sale_item id
	Name        string `json:"name"`
	UnitAbbr    string `json:"unitAbbr"`
	Quantity    string `json:"quantity"`
	ReturnedQty string `json:"returnedQty"`
	UnitPrice   string `json:"unitPrice"`
	Subtotal    string `json:"subtotal"`   // line net (after line discount)
	RefundUnit  string `json:"refundUnit"` // what one returned unit refunds (subtotal / qty, 2dp)
	IsService   bool   `json:"isService"`  // never returnable
}

type SalePayment struct {
	Method string `json:"method"`
	Amount string `json:"amount"`
}

type SaleView struct {
	SaleSummary `tstype:",extends"`
	CustomerID  int64         `json:"customerId"` // 0 = walk-in
	Subtotal    string        `json:"subtotal"`
	Discount    string        `json:"discount"`
	Tax         string        `json:"tax"`
	PaidAmount  string        `json:"paidAmount"`
	ChangeGiven string        `json:"changeGiven"`
	OwedOnSale  string        `json:"owedOnSale"` // total − paid when a customer is on it; a return clears this first
	Lines       []SaleLine    `json:"lines"`
	Payments    []SalePayment `json:"payments"`
}

// ---- Returns ----

type ReturnLine struct {
	SaleItemID  int64  `json:"saleItemId"`
	Quantity    string `json:"quantity"`
	Disposition string `json:"disposition"` // restock (default) | damage
}

type ReturnRequest struct {
	Reason          string       `json:"reason"`
	Lines           []ReturnLine `json:"lines"`
	DeferDrawerKick bool         `json:"deferDrawerKick"`
}

type ReturnResult struct {
	CashRefund string `json:"cashRefund"`
}

// ---- Credit ----

type CreditPaymentRequest struct {
	Note            string `json:"note"`
	Amount          string `json:"amount"`
	Method          string `json:"method"`    // cash | card | online
	AccountID       int64  `json:"accountId"` // card/bank locker the money lands in
	Reference       string `json:"reference"`
	DeferDrawerKick bool   `json:"deferDrawerKick"`
}

type CreditPaymentResult struct {
	PaymentID    int64  `json:"paymentId"` // for the debt slip
	ReceiptNo    string `json:"receiptNo"`
	BalanceAfter string `json:"balanceAfter"`
}

type AdvanceRefundRequest struct {
	Amount          string `json:"amount"`
	DeferDrawerKick bool   `json:"deferDrawerKick"`
}

// ---- Receipts hub ----

type MoneyReceiptRow struct {
	ID        int64  `json:"id"`
	ReceiptNo string `json:"receiptNo"`
	Kind      string `json:"kind"`
	KindLabel string `json:"kindLabel"`
	FromLabel string `json:"fromLabel"`
	ToLabel   string `json:"toLabel"`
	Party     string `json:"party"`
	Amount    string `json:"amount"`
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
	CreatedBy string `json:"createdBy"`
}

type DebtReceiptRow struct {
	ID            int64  `json:"id"`
	ReceiptNo     string `json:"receiptNo"`
	CreatedAt     string `json:"createdAt"`
	CustomerName  string `json:"customerName"`
	CustomerPhone string `json:"customerPhone"`
	Amount        string `json:"amount"`
	Method        string `json:"method"`
	BalanceBefore string `json:"balanceBefore"` // "" = not recorded
	BalanceAfter  string `json:"balanceAfter"`
	CashierName   string `json:"cashierName"`
}

// ---- Z report (day-end for one drawer session) ----

type ZMethod struct {
	Method string `json:"method"`
	Amount string `json:"amount"`
}

type ZMove struct {
	At     string `json:"at"`
	Type   string `json:"type"` // opening | sale | credit_payment | withdrawal | expense | pay_in | …
	Reason string `json:"reason"`
	Amount string `json:"amount"`
}

type ZFloatItem struct {
	Name    string `json:"name"`
	Balance string `json:"balance"`
}

type ZFloatGroup struct {
	Group string       `json:"group"`
	Items []ZFloatItem `json:"items"`
	Total string       `json:"total"`
}

type ZReport struct {
	SessionID    int64         `json:"sessionId"`
	CashierName  string        `json:"cashierName"`
	OpenedAt     string        `json:"openedAt"`
	ClosedAt     string        `json:"closedAt"` // "" = still open (an X read)
	OpeningCash  string        `json:"openingCash"`
	ExpectedCash string        `json:"expectedCash"` // "" until closed
	ClosingCash  string        `json:"closingCash"`
	Difference   string        `json:"difference"`
	SalesCount   int           `json:"salesCount"`
	Gross        string        `json:"gross"`
	Discount     string        `json:"discount"`
	Returns      string        `json:"returns"`
	Net          string        `json:"net"`
	ByMethod     []ZMethod     `json:"byMethod"`
	Moves        []ZMove       `json:"moves"`
	Floats       []ZFloatGroup `json:"floats"` // plugin money locations (M7)
}

// ---- Warranty ----

type WarrantyClaimRow struct {
	ID                int64  `json:"id"`
	At                string `json:"at"`
	Reason            string `json:"reason"`
	Resolution        string `json:"resolution"` // replaced | …
	ProductName       string `json:"productName"`
	OldSerial         string `json:"oldSerial"`
	ReplacementSerial string `json:"replacementSerial"` // "" = no new unit (receipt claim)
	CustomerName      string `json:"customerName"`
	HandledBy         string `json:"handledBy"`
}

type WarrantyUnitRow struct {
	ID            int64  `json:"id"`
	Serial        string `json:"serial"`
	ProductName   string `json:"productName"`
	CustomerName  string `json:"customerName"`
	ReceiptNo     string `json:"receiptNo"`
	SoldAt        string `json:"soldAt"`
	Months        int    `json:"months"`
	Until         string `json:"until"`  // YYYY-MM-DD
	Left          string `json:"left"`   // "11 mo left" | "<1 mo left" | "expired"
	Status        string `json:"status"` // active | replaced
	UnderWarranty bool   `json:"underWarranty"`
}

type WarrantyUnitView struct {
	WarrantyUnitRow `tstype:",extends"`
	Claims          []WarrantyClaimRow `json:"claims"`
}

type WarrantyReceiptLine struct {
	ProductID     int64  `json:"productId"`
	Name          string `json:"name"`
	UnitAbbr      string `json:"unitAbbr"`
	Qty           string `json:"qty"`
	Replaced      int    `json:"replaced"`
	Remaining     string `json:"remaining"` // units still claimable
	Months        int    `json:"months"`
	Until         string `json:"until"`
	UnderWarranty bool   `json:"underWarranty"`
}

type WarrantyReceiptView struct {
	SaleID       int64                 `json:"saleId"`
	ReceiptNo    string                `json:"receiptNo"`
	SoldAt       string                `json:"soldAt"`
	CustomerName string                `json:"customerName"`
	Lines        []WarrantyReceiptLine `json:"lines"`
}

type WarrantyReplaceRequest struct {
	UnitID    int64  `json:"unitId"`
	NewSerial string `json:"newSerial"`
	Reason    string `json:"reason"`
}

type WarrantyReceiptReplaceRequest struct {
	SaleID    int64  `json:"saleId"`
	ProductID int64  `json:"productId"`
	Reason    string `json:"reason"`
}

type WarrantyClaimRef struct {
	ClaimID int64 `json:"claimId"`
}

// ---- Suppliers at the counter ----

type SupplierRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Contact    string `json:"contact"`
	Owed       string `json:"owed"`    // what the shop owes them (negative = they hold our advance)
	OldDebt    string `json:"oldDebt"` // opening balance not tied to an invoice
	CreditDays int    `json:"creditDays"`
}

type SupplierInvoice struct {
	ID        int64  `json:"id"`
	InvoiceNo string `json:"invoiceNo"`
	Date      string `json:"date"`
	DueDate   string `json:"dueDate"` // "" = none
	Total     string `json:"total"`
	Balance   string `json:"balance"`
}

type SupplierOrder struct {
	ID    int64  `json:"id"`
	Date  string `json:"date"`
	Total string `json:"total"`
	Notes string `json:"notes"`
}

type SupplierView struct {
	SupplierRow `tstype:",extends"`
	Invoices    []SupplierInvoice `json:"invoices"` // open, oldest first
	Orders      []SupplierOrder   `json:"orders"`   // drafts waiting to be received
}

// SupplierPayRequest pays one amount, spread by the forked waterfall:
// Mode "pay" = open invoices oldest-first, "old" = old debt first; the rest is an advance.
type SupplierPayRequest struct {
	CounterMoneyRequest `tstype:",extends"`
	Amount              string `json:"amount"`
	Mode                string `json:"mode"`
	Method              string `json:"method"` // cash | card | online
	Reference           string `json:"reference"`
	Note                string `json:"note"`
}

// SupplierRefundRequest takes money back from a supplier (into the drawer or a locker).
type SupplierRefundRequest struct {
	CounterMoneyRequest `tstype:",extends"`
	Amount              string `json:"amount"`
	Method              string `json:"method"`
	Reference           string `json:"reference"`
	Note                string `json:"note"`
}

// ---- Receiving stock and orders at the counter ----

type StockLine struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Ordered      string `json:"ordered"` // on an order; "" for walk-in deliveries
	Quantity     string `json:"quantity"`
	FreeQty      string `json:"freeQty"`
	CostPrice    string `json:"costPrice"`
	SellingPrice string `json:"sellingPrice"`
	ExpiryDate   string `json:"expiryDate"`   // YYYY-MM-DD or ""
	Discount     string `json:"discount"`     // what the supplier took off this line
	DiscountType string `json:"discountType"` // fixed = Rs per unit · percent = off qty × cost
}

type OrderView struct {
	ID           int64       `json:"id"`
	SupplierID   int64       `json:"supplierId"`
	SupplierName string      `json:"supplierName"`
	Date         string      `json:"date"`
	Status       string      `json:"status"` // draft until received
	Notes        string      `json:"notes"`
	Lines        []StockLine `json:"lines"`
}

// PayNow settles part of a delivery on the spot (clamped to what's owed on it).
type PayNow struct {
	Amount   string `json:"amount"` // "" / 0 = all on credit
	Method   string `json:"method"`
	LockerID int64  `json:"lockerId"`
}

type ReceiveRequest struct {
	InvoiceNo       string      `json:"invoiceNo"`
	Discount        string      `json:"discount"`     // whole-bill discount
	DiscountType    string      `json:"discountType"` // fixed = Rs off · percent = off the net after line discounts
	Notes           string      `json:"notes"`
	Lines           []StockLine `json:"lines"`
	KeepRemainder   bool        `json:"keepRemainder"` // against an order: the unreceived rest stays on order
	Pay             PayNow      `json:"pay"`
	DeferDrawerKick bool        `json:"deferDrawerKick"`
}

type ReceiveResult struct {
	PurchaseID int64  `json:"purchaseId"`
	Total      string `json:"total"`
	Paid       string `json:"paid"`
	ReceiptID  *int64 `json:"receiptId,omitempty"` // CR- slip when cash left the counter
}

type OrderRequest struct {
	WantedBy string      `json:"wantedBy"` // YYYY-MM-DD or "" (the order's expected date)
	Notes    string      `json:"notes"`
	Lines    []StockLine `json:"lines"`
}

type NewSupplierRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// IntakeProductRequest creates a product first met on a delivery (Wanted = while
// taking an order: prices are never taken from a quote).
type IntakeProductRequest struct {
	Name         string `json:"name"`
	Barcode      string `json:"barcode"`
	CostPrice    string `json:"costPrice"`
	SellingPrice string `json:"sellingPrice"`
	UnitID       int64  `json:"unitId"`
	Wanted       bool   `json:"wanted"`
}
