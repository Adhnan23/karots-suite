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
