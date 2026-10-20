package api

type DrawerSession struct {
	ID          int64  `json:"id"`
	OpeningCash string `json:"openingCash"`
	OpenedAt    string `json:"openedAt"` // RFC3339
}

// DrawerSummary is this user's till session. LastBreakdown is the previous close's
// {denominationValue: count}, for "Continue last count".
type DrawerSummary struct {
	Session       *DrawerSession     `json:"session,omitempty"`
	CashSales     string             `json:"cashSales"`
	PayIns        string             `json:"payIns"`
	Withdrawals   string             `json:"withdrawals"`
	Expected      string             `json:"expected"`
	LastClosing   *string            `json:"lastClosing,omitempty"`
	LastBreakdown map[string]float64 `json:"lastBreakdown,omitempty"`
	// Stale: the open session is from an earlier day — the till makes them close it first.
	Stale bool `json:"stale"`
}

type Denomination struct {
	Value  string `json:"value"` // decimal string as the breakdown key, e.g. "500"
	IsNote bool   `json:"isNote"`
}

// Locker is a cashier-accessible money account: cash lockers (safe/pocket/other)
// for drawer moves, card/bank ones as card/online tender destinations.
type Locker struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // safe | pocket | other | card | bank
	Balance string `json:"balance"`
}

type DrawerState struct {
	Summary       DrawerSummary  `json:"summary"`
	Denominations []Denomination `json:"denominations"`
	Lockers       []Locker       `json:"lockers"`
}

type OpenDrawerRequest struct {
	Breakdown      map[string]float64 `json:"breakdown"`
	SourceLockerID int64              `json:"sourceLockerId"` // 0 = untracked
}

type CloseDrawerRequest struct {
	Breakdown    map[string]float64 `json:"breakdown"`
	DestLockerID int64              `json:"destLockerId"` // 0 = leave in drawer (if allowed)
}

type CloseResult struct {
	SessionID    int64  `json:"sessionId"`
	CashSales    string `json:"cashSales"`
	ExpectedCash string `json:"expectedCash"`
	Counted      string `json:"counted"`
	Difference   string `json:"difference"`
	ReceiptID    *int64 `json:"receiptId,omitempty"`
}

type MoneyMoveRequest struct {
	Amount          string `json:"amount"`
	Reason          string `json:"reason"`
	LockerID        int64  `json:"lockerId"`
	DeferDrawerKick bool   `json:"deferDrawerKick"` // auto-print folds the pulse into the slip job
}

// ReceiptRef points at the CR- money receipt a drawer move produced (if any).
type ReceiptRef struct {
	ReceiptID *int64 `json:"receiptId,omitempty"`
}
