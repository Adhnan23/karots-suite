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
