package api

// OpeningRequest corrects the still-unpaid old (pre-system) balance of a
// customer or supplier. Negative = a pre-system credit / advance.
type OpeningRequest struct {
	Opening string `json:"opening"`
	Note    string `json:"note"`
}

// StatementLine is one ledger line; Balance runs forward.
type StatementLine struct {
	Date    string `json:"date"`
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Debit   string `json:"debit"`
	Credit  string `json:"credit"`
	Balance string `json:"balance"`
}

// Statement is a customer's or supplier's printable ledger.
type Statement struct {
	Name        string          `json:"name"`
	Phone       string          `json:"phone"`
	Address     string          `json:"address"`
	Contact     string          `json:"contact"`
	Since       string          `json:"since"`
	Lines       []StatementLine `json:"lines"`
	TotalDebit  string          `json:"totalDebit"`
	TotalCredit string          `json:"totalCredit"`
	Balance     string          `json:"balance"`
}

// AdminCustomerRow is one customer on the admin list. Owed < 0 = an advance
// they hold with the shop. OldDebt is the correctable pre-system part.
type AdminCustomerRow struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	CreditLimit string `json:"creditLimit"`
	Owed        string `json:"owed"`
	OldDebt     string `json:"oldDebt"`
	Loyalty     int    `json:"loyalty"`
	OverLimit   bool   `json:"overLimit"`
	Active      bool   `json:"active"`
}

type AdminCustomerList struct {
	Items    []AdminCustomerRow `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	// Owed / Advance sum every matching customer.
	Owed    string `json:"owed"`
	Advance string `json:"advance"`
}

// AdminCustomerDetail adds the customer's recent ledger, newest first.
type AdminCustomerDetail struct {
	AdminCustomerRow `tstype:",extends"`
	Since            string          `json:"since"`
	Recent           []StatementLine `json:"recent"`
}

// CustomerRequest creates or edits a customer; OpeningBalance is read on create only.
type CustomerRequest struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Address        string `json:"address"`
	CreditLimit    string `json:"creditLimit"`
	OpeningBalance string `json:"openingBalance"`
}

// CustomerPayRequest collects a credit repayment in the back office. Source is
// a MoneySource value: cash needs one; card / online may name the card / bank
// account it lands in ("" = recorded only).
type CustomerPayRequest struct {
	Amount       string `json:"amount"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Note         string `json:"note"`
	Source       string `json:"source"`
	OldDebtFirst bool   `json:"oldDebtFirst"`
}
