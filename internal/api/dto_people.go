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
// a MoneySource value: a cash locker or till for cash, the card / bank account
// for card / online.
type CustomerPayRequest struct {
	Amount       string `json:"amount"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	Note         string `json:"note"`
	Source       string `json:"source"`
	OldDebtFirst bool   `json:"oldDebtFirst"`
	// Meta is a plugin tender's detail (Method = a plugin tender; Source unused).
	Meta map[string]string `json:"meta,omitempty"`
	// Cheques are the cheques of a Method "cheque" payment; Amount is their total.
	Cheques []ChequeLine `json:"cheques,omitempty"`
}

// CustomerRefundRequest hands a customer's advance back in cash from Source (a
// cash locker or till MoneySource value).
type CustomerRefundRequest struct {
	Amount string `json:"amount"`
	Source string `json:"source"`
}

// StaffUser is a login on the Users page (the hidden system user never is).
type StaffUser struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Phone              string `json:"phone"`
	Role               string `json:"role"` // admin | manager | cashier
	Active             bool   `json:"active"`
	MustChangePin      bool   `json:"mustChangePin"`
	ReceiptPrinter     string `json:"receiptPrinter"`
	CanHandleSuppliers bool   `json:"canHandleSuppliers"`
	CanManageCredit    bool   `json:"canManageCredit"`
	CanSeeCost         bool   `json:"canSeeCost"`
	CanSeeLowStock     bool   `json:"canSeeLowStock"`
	Since              string `json:"since"`
}

// StaffRequest creates or edits a login. Pin is required on create; on edit a
// blank Pin keeps the current one (a new one signs them out everywhere).
type StaffRequest struct {
	Name               string `json:"name"`
	Phone              string `json:"phone"`
	Role               string `json:"role"`
	Pin                string `json:"pin"`
	ReceiptPrinter     string `json:"receiptPrinter"`
	CanHandleSuppliers bool   `json:"canHandleSuppliers"`
	CanManageCredit    bool   `json:"canManageCredit"`
	CanSeeCost         bool   `json:"canSeeCost"`
	CanSeeLowStock     bool   `json:"canSeeLowStock"`
}

// PresenceBeat is a signed-in screen's heartbeat (every 20 s, on route change and
// focus); the device comes from the X-Ks-Device header, locked from the token.
type PresenceBeat struct {
	Path      string  `json:"path"`
	IdleSec   int     `json:"idleSec"`
	CartItems float64 `json:"cartItems"`
	CartTotal string  `json:"cartTotal"`
	Note      string  `json:"note"` // e.g. "Playing Blocks"
}

// PresenceReply answers a heartbeat: lock = an admin asked this screen to lock now.
type PresenceReply struct {
	Lock bool `json:"lock"`
}

// PresenceRow is one device on the admin's "Who's on now".
type PresenceRow struct {
	Device     string  `json:"device"`
	UserID     int64   `json:"userId"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	State      string  `json:"state"` // active | idle | locked | away | signedOut
	Path       string  `json:"path"`
	IdleSec    int     `json:"idleSec"`
	CartItems  float64 `json:"cartItems"`
	CartTotal  string  `json:"cartTotal"`
	Note       string  `json:"note"`
	IP         string  `json:"ip"`
	Label      string  `json:"label"` // "Windows · Chrome"
	LastSeen   string  `json:"lastSeen"`
	TillOpened *string `json:"tillOpened"` // their open till's opening time, null = none
	TillStale  bool    `json:"tillStale"`  // that till is from an earlier day
	SalesToday int     `json:"salesToday"`
	// Serving: the cart changed in the last 5 min — no remote lock.
	Serving bool `json:"serving"`
	// CartUntouchedMin: minutes the cart has sat unchanged, once 10+ (else 0).
	CartUntouchedMin int  `json:"cartUntouchedMin"`
	LockPending      bool `json:"lockPending"`
}
