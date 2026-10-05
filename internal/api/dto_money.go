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

// LockerTransferRequest moves money from one locker to another.
type LockerTransferRequest struct {
	FromID int64  `json:"fromId"`
	ToID   int64  `json:"toId"`
	Amount string `json:"amount"`
	Note   string `json:"note"`
}

// LockerAdjustRequest is a bank charge (out, books an expense), interest (in),
// or a manual count correction up / down (note required).
type LockerAdjustRequest struct {
	Type   string `json:"type"` // bank_charge | interest | adjust_up | adjust_down
	Amount string `json:"amount"`
	Note   string `json:"note"`
}

// SettleCardRequest banks a card-machine batch: Gross leaves the card account,
// Gross − Fee lands in the bank, and Fee is booked as an expense.
type SettleCardRequest struct {
	CardID int64  `json:"cardId"`
	BankID int64  `json:"bankId"`
	Gross  string `json:"gross"`
	Fee    string `json:"fee"`
}

// AdminExpenseRow is one expense on Money → Expenses. PaidFrom is where its
// cash left ("" = outside the shop, no money moved); a paid expense's amount
// can't be edited, only voided.
type AdminExpenseRow struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"` // YYYY-MM-DD
	Category    string `json:"category"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	PaidBy      string `json:"paidBy"`
	ServiceID   int64  `json:"serviceId"`
	ServiceName string `json:"serviceName"`
	PaidFrom    string `json:"paidFrom"`
}

// ExpenseCategoryTotal is one category chip: its share of the range.
type ExpenseCategoryTotal struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
	Total    string `json:"total"`
}

// AdminExpenseSummary heads the expenses list: the total of every match (from
// SQL, not the loaded page), the range's per-category totals, and the form's
// category suggestions and taggable services.
type AdminExpenseSummary struct {
	Total      string                 `json:"total"`
	Count      int                    `json:"count"`
	ByCategory []ExpenseCategoryTotal `json:"byCategory"`
	Categories []string               `json:"categories"`
	Services   []ServiceOption        `json:"services"`
}

// AdminExpenseRequest records or edits an expense. Source ("locker:3",
// "till:5", "" = outside the shop) is read on create only; on edit Amount
// may change only while no money moved for it.
type AdminExpenseRequest struct {
	Category    string `json:"category"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	Date        string `json:"date"` // YYYY-MM-DD; "" = today
	ServiceID   int64  `json:"serviceId"`
	Source      string `json:"source"`
}

// ExpenseVoidInfo prepares the void sheet: whether money moved for it and from
// where, the place to put it back (pre-picked: the locker it left, if still in
// use), and the plugin record that owns it ("" = none; a claimed one can't be
// voided here).
type ExpenseVoidInfo struct {
	Paid   bool   `json:"paid"`
	From   string `json:"from"`
	Source string `json:"source"`
	Owner  string `json:"owner"`
}

// ExpenseVoidRequest is where a paid expense's money goes back.
type ExpenseVoidRequest struct {
	Dest string `json:"dest"`
}
