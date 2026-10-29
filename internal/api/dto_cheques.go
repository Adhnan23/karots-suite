package api

// ChequeLine is one cheque in a payment form. Received: Bank is the payer's
// bank. Given: LockerID is the shop's bank place it is drawn on.
type ChequeLine struct {
	Bank     string `json:"bank"`
	LockerID int64  `json:"lockerId"`
	Number   string `json:"number"`
	Amount   string `json:"amount"`
	Date     string `json:"date"` // YYYY-MM-DD
	Note     string `json:"note"`
}

// ChequeRow is one cheque on the Cheques page.
type ChequeRow struct {
	ID         int64  `json:"id"`
	Direction  string `json:"direction"` // in = received, out = given
	ChequeDate string `json:"chequeDate"`
	Who        string `json:"who"` // customer, supplier, or the expense
	Bank       string `json:"bank"`
	LockerID   int64  `json:"lockerId"` // given: drawn on; received: where it cleared (0 = not yet)
	LockerName string `json:"lockerName"`
	Number     string `json:"number"`
	Amount     string `json:"amount"`
	Status     string `json:"status"` // pending | cleared | bounced | cancelled
	StatusDate string `json:"statusDate"`
	Days       int    `json:"days"` // until (+) / since (−) the cheque date
}

type CountTotal struct {
	Count int    `json:"count"`
	Total string `json:"total"`
}

// BankAfter is a bank place whose pending given cheques take it below zero.
type BankAfter struct {
	LockerID int64  `json:"lockerId"`
	Name     string `json:"name"`
	After    string `json:"after"`
}

// ChequeSummary heads the Cheques page and feeds the dashboard reminders.
type ChequeSummary struct {
	InHand    string      `json:"inHand"`    // pending received (Cheques in hand)
	Given     string      `json:"given"`     // pending given (to be cashed)
	DueWeek   string      `json:"dueWeek"`   // pending given dated within 7 days or past
	ToDeposit CountTotal  `json:"toDeposit"` // received, pending, dated today or earlier
	GivenDue  CountTotal  `json:"givenDue"`  // given, pending, dated within 3 days or past
	Short     []BankAfter `json:"short"`
}

// ChequeSource is the record a cheque paid.
type ChequeSource struct {
	Kind  string `json:"kind"` // sale | customer_payment | supplier_payment | expense | ""
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type ChequeEvent struct {
	At     string `json:"at"`
	User   string `json:"user"`
	Detail string `json:"detail"`
}

type ChequeDetail struct {
	ChequeRow  `tstype:",extends"`
	Note       string        `json:"note"`
	CustomerID int64         `json:"customerId"`
	SupplierID int64         `json:"supplierId"`
	Source     ChequeSource  `json:"source"`
	History    []ChequeEvent `json:"history"`
	// LastBankID is the bank place the last received cheque cleared into (pre-fills Cleared…).
	LastBankID int64 `json:"lastBankId"`
}

type ChequeClearRequest struct {
	Date     string `json:"date"` // YYYY-MM-DD; "" = today
	LockerID int64  `json:"lockerId"`
}

type ChequeReturnRequest struct {
	Status string `json:"status"` // bounced | cancelled
}

type ChequeFixRequest struct {
	Bank     string `json:"bank"`
	LockerID int64  `json:"lockerId"`
	Number   string `json:"number"`
	Date     string `json:"date"`
	Note     string `json:"note"`
}

type ChequeDuplicate struct {
	Duplicate bool `json:"duplicate"`
}
