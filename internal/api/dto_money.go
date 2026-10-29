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
	Unpaid      string `json:"unpaid"` // > 0: its cheque bounced / was cancelled — pay it again
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
	ServiceID   int64        `json:"serviceId"`
	Source      string       `json:"source"`
	Method      string       `json:"method,omitempty"`  // "" = from Source; cheque = Cheques
	Cheques     []ChequeLine `json:"cheques,omitempty"` // Method cheque: adding up to Amount
}

// AdminExpensePayRequest pays what is still unpaid on an expense whose cheque
// bounced or was cancelled.
type AdminExpensePayRequest struct {
	Method  string       `json:"method"` // cash | card | online | cheque
	Source  string       `json:"source"`
	Cheques []ChequeLine `json:"cheques,omitempty"`
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

// CashPlace is one place's live balance on Cash flow. Value is the MoneySource
// value for the ledger's place filter ("" for a plugin location).
type CashPlace struct {
	Value   string `json:"value"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // locker kind, "till", or "plugin"
	Balance string `json:"balance"`
}

// CashPlaceGroup is a plugin's money locations (e.g. a carrier's float):
// owned by the shop but not cash on hand.
type CashPlaceGroup struct {
	Group string      `json:"group"`
	Items []CashPlace `json:"items"`
	Total string      `json:"total"`
}

// NetLine is one line of the net-position card. Sign +1 adds, −1 subtracts.
type NetLine struct {
	Label  string `json:"label"`
	Amount string `json:"amount"`
	Sign   int    `json:"sign"`
}

// AdminCashflow is the Cash flow page (karots-pos /admin/cashflow, with its
// 9f17be1 fix): live balances, the range's totals from SQL over the whole range
// (moves between the shop's own places counted once, apart from in / out), and
// the net position line by line.
type AdminCashflow struct {
	From        string           `json:"from"` // YYYY-MM-DD, inclusive
	To          string           `json:"to"`
	Places      []CashPlace      `json:"places"`
	Plugins     []CashPlaceGroup `json:"plugins"`
	CashOnHand  string           `json:"cashOnHand"`
	TillSales   string           `json:"tillSales"`
	In          string           `json:"in"`
	Openings    string           `json:"openings"` // opening balances of lockers added in range (not money in)
	Out         string           `json:"out"`
	Moved       string           `json:"moved"`
	Net         string           `json:"net"`
	Rows        int              `json:"rows"`
	NetLines    []NetLine        `json:"netLines"`
	NetPosition string           `json:"netPosition"`
}

// CashflowRow is one ledger line. Moved = one side of a move between the
// shop's own places (left out of in / out).
type CashflowRow struct {
	At        string `json:"at"`
	Place     string `json:"place"`
	Location  string `json:"location"`
	Kind      string `json:"kind"`
	Delta     string `json:"delta"`
	Note      string `json:"note"`
	Moved     bool   `json:"moved"`
	ReceiptID int64  `json:"receiptId"`
	ReceiptNo string `json:"receiptNo"`
}

// TillSessionRow is one till session (karots-pos /admin/cash-register).
// Expected is live for an open till; Counted / Difference are "" until it
// closes. Difference < 0 = short, > 0 = over.
type TillSessionRow struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	Cashier    string `json:"cashier"`
	OpenedAt   string `json:"openedAt"`
	ClosedAt   string `json:"closedAt"`
	Open       bool   `json:"open"`
	Opening    string `json:"opening"`
	Expected   string `json:"expected"`
	Counted    string `json:"counted"`
	Difference string `json:"difference"`
	// Stale: open since an earlier day. ClosedLate: closed on a later day than it opened.
	Stale      bool `json:"stale"`
	ClosedLate bool `json:"closedLate"`
	// ClosedBy names who closed it for its cashier ("" = the cashier did).
	ClosedBy string `json:"closedBy"`
}

// TillMove is one drawer movement in a session.
type TillMove struct {
	At     string `json:"at"`
	Type   string `json:"type"` // opening | pay_in | withdrawal | expense | credit_payment | refund | closing
	Amount string `json:"amount"`
	Reason string `json:"reason"`
}

// TillSessionView is one session: its money, every movement, and the drawer
// counts by denomination ({value: count}) at open and close.
type TillSessionView struct {
	Session      TillSessionRow     `json:"session"`
	Moves        []TillMove         `json:"moves"`
	OpeningCount map[string]float64 `json:"openingCount"`
	ClosingCount map[string]float64 `json:"closingCount"`
}

// CashierOption is someone who has opened a till (the list's filter).
type CashierOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AdminDenomination is a note or coin the drawer count offers; Active = still
// in circulation (shown in the count).
type AdminDenomination struct {
	ID     int64  `json:"id"`
	Value  string `json:"value"`
	IsNote bool   `json:"isNote"`
	Active bool   `json:"active"`
}

// DenominationRequest adds or edits one.
type DenominationRequest struct {
	Value  string `json:"value"`
	IsNote bool   `json:"isNote"`
	Active bool   `json:"active"`
}

// AdminSaleRow is one sale on Sell → Sales. Paid = tendered less change;
// Profit is after cost, returns and pass-through (admins and managers always
// see cost).
type AdminSaleRow struct {
	SaleSummary `tstype:",extends"`
	Paid        string `json:"paid"`
	COGS        string `json:"cogs"`
	Profit      string `json:"profit"`
	Margin      string `json:"margin"` // percent of net revenue, 1 dp
}

// AdminSalesSummary covers every sale the filters match (SQL, not the page).
type AdminSalesSummary struct {
	Count  int    `json:"count"`
	Total  string `json:"total"`
	Paid   string `json:"paid"`
	Profit string `json:"profit"`
	Margin string `json:"margin"` // percent, 1 dp
}

// AdminReturnRequest is a return from the admin: the counter's lines plus
// Source, where any cash refund leaves from (a MoneySource value; required
// only when there is cash to give back).
type AdminReturnRequest struct {
	Reason string       `json:"reason"`
	Lines  []ReturnLine `json:"lines"`
	Source string       `json:"source"`
}

// AdminReturnResult is the cash refunded and its CR- receipt (nil = no cash).
type AdminReturnResult struct {
	CashRefund string `json:"cashRefund"`
	ReceiptID  *int64 `json:"receiptId,omitempty"`
}

// FinanceLine is one P&L line or one labelled amount.
type FinanceLine struct {
	Label  string `json:"label"`
	Amount string `json:"amount"`
}

// FinancePoint is one period on the trend (Day = period start, YYYY-MM-DD).
type FinancePoint struct {
	Day     string `json:"day"`
	Revenue string `json:"revenue"`
	Profit  string `json:"profit"`
}

// FinanceSlice is one payment method's share of takings.
type FinanceSlice struct {
	Method string `json:"method"`
	Count  int    `json:"count"`
	Total  string `json:"total"`
}

// FinanceCategory is one category's revenue and profit.
type FinanceCategory struct {
	Category string `json:"category"`
	Revenue  string `json:"revenue"`
	Profit   string `json:"profit"`
}

// FinanceProduct is a top seller.
type FinanceProduct struct {
	Name    string `json:"name"`
	Qty     string `json:"qty"`
	Revenue string `json:"revenue"`
	Profit  string `json:"profit"`
}

// AdminFinance is the Finance hub (karots-pos /admin/finance, its Overview,
// Profit and Cash tabs in one read): the P&L for the range (plugin income lines
// folded into net profit), the trend at a grain fitting the range, the payment
// mix, profit by category, top products, and the money owed either way.
type AdminFinance struct {
	From            string            `json:"from"`
	To              string            `json:"to"`
	Grain           string            `json:"grain"` // day | week | month
	SalesCount      int               `json:"salesCount"`
	GrossRevenue    string            `json:"grossRevenue"`
	Returns         string            `json:"returns"`
	Revenue         string            `json:"revenue"`
	COGS            string            `json:"cogs"`
	GrossProfit     string            `json:"grossProfit"`
	GrossMargin     string            `json:"grossMargin"` // percent, 1 dp
	Expenses        string            `json:"expenses"`
	Losses          string            `json:"losses"`
	OwnUse          string            `json:"ownUse"`
	StaffWelfare    string            `json:"staffWelfare"`
	StockCorrection string            `json:"stockCorrections"`
	Recoveries      string            `json:"recoveries"`
	OtherIncome     string            `json:"otherIncome"`
	PluginIncome    []FinanceLine     `json:"pluginIncome"`
	NetProfit       string            `json:"netProfit"`
	ZeroCostLines   int               `json:"zeroCostLines"`
	ZeroCostRevenue string            `json:"zeroCostRevenue"`
	Received        string            `json:"received"`
	Tax             string            `json:"tax"` // tax charged on the period's sales — owed to the tax office, not revenue
	CashWithdrawn   string            `json:"cashWithdrawn"`
	RegisterDiff    string            `json:"registerDiff"`
	Receivables     string            `json:"receivables"`
	Advances        string            `json:"customerAdvances"`
	Payables        string            `json:"payables"`
	SupplierCredit  string            `json:"supplierCredit"`
	Trend           []FinancePoint    `json:"trend"`
	ByPayment       []FinanceSlice    `json:"byPayment"`
	Categories      []FinanceCategory `json:"categories"`
	TopProducts     []FinanceProduct  `json:"topProducts"`
}
