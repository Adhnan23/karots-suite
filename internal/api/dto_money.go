package api

// AdminLocker is one locker on the Money → Lockers page (karots-pos LockersPage):
// its live balance, whether it may go below zero, whether cashiers may use it
// from the till, and whether the till's dialogs pre-select it.
type AdminLocker struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"` // safe | bank | pocket | card | other
	Balance       string `json:"balance"`
	Overdraft     bool   `json:"overdraft"`
	CashierAccess bool   `json:"cashierAccess"`
	Active        bool   `json:"active"`
	TillDefault   bool   `json:"tillDefault"`
}

// AdminLockerList is the lockers page: every locker, the total across the
// active ones, and the till's cash settings that live on the same page.
type AdminLockerList struct {
	Items    []AdminLocker   `json:"items"`
	Total    string          `json:"total"`
	TillCash TillCashRequest `json:"tillCash"`
}

// LockerRequest creates or edits a locker. OpeningBalance is read on create
// only (an open_balance ledger row); Active on edit only.
type LockerRequest struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	OpeningBalance string `json:"openingBalance"`
	Overdraft      bool   `json:"overdraft"`
	CashierAccess  bool   `json:"cashierAccess"`
}

// TillCashRequest is the till's cash-move settings: the locker its open /
// deposit / withdraw / close dialogs pre-select (0 = none), and whether cash
// may leave the drawer without a locker (karots-pos cashier-default-locker).
type TillCashRequest struct {
	DefaultLockerID int64 `json:"defaultLockerId"`
	AllowUntracked  bool  `json:"allowUntracked"`
}

// AdminLockerView is one locker's ledger page header: the locker and how much
// its balance moved over the whole chosen range.
type AdminLockerView struct {
	Locker AdminLocker `json:"locker"`
	Net    string      `json:"net"`
}

// LockerLedgerRow is one movement on a locker's ledger. Delta > 0 = money in.
// ReceiptID is the CR- receipt its move wrote (0 = none found).
type LockerLedgerRow struct {
	ID        int64  `json:"id"`
	At        string `json:"at"`
	Kind      string `json:"kind"` // open_balance | transfer | payment | intake | bank_charge | interest | adjust
	Note      string `json:"note"`
	Delta     string `json:"delta"`
	ReceiptID int64  `json:"receiptId"`
	ReceiptNo string `json:"receiptNo"`
}
