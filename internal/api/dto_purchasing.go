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
	Source      string     `json:"source"` // MoneySource value; "" = outside the shop (not for cash)
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
