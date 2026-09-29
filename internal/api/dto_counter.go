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
