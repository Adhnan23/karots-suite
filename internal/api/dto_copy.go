package api

// TillCopy is everything a till needs to sell with no server: read in one go
// by a client that keeps it (offline), refreshed when Version changes. Only
// cashier and manager PIN hashes are in it — never an admin's or the system user's.
type TillCopy struct {
	TakenAt            string         `json:"takenAt"`
	Version            string         `json:"version"`
	Shop               ShopHeader     `json:"shop"`
	Symbol             string         `json:"symbol"`
	DefaultSaleType    string         `json:"defaultSaleType"`
	AskToPrint         bool           `json:"askToPrint"`
	AllowUntrackedCash bool           `json:"allowUntrackedCash"`
	DefaultLockerID    int64          `json:"defaultLockerId"`
	TenderMethods      []TenderMethod `json:"tenderMethods"`
	Products           []CopyProduct  `json:"products"`
	Categories         []CopyCategory `json:"categories"`
	Customers          []CopyCustomer `json:"customers"`
	People             []CopyPerson   `json:"people"`
	Places             []CopyPlace    `json:"places"`
	Sessions           []CopySession  `json:"sessions"`
	Held               []CopyHeld     `json:"held"`
	// PriceOptions is what /till/price-options answers (product id → lots), so
	// Which price? works offline.
	PriceOptions  map[string][]Lot `json:"priceOptions"`
	Denominations []Denomination   `json:"denominations"` // the shop's active notes and coins
	// LockTimeoutMinutes: idle minutes before the phone locks (0 = never), as the PC.
	LockTimeoutMinutes int  `json:"lockTimeoutMinutes"`
	AllowContinueLast  bool `json:"allowContinueLast"` // the drawer may start from the last close's count
}

// CopyProduct is a till card plus what an offline search needs: its category
// and every code it scans by (packs carry their own).
type CopyProduct struct {
	TillProduct `tstype:",extends"`
	CategoryID  int64      `json:"categoryId"`
	Barcodes    []string   `json:"barcodes"` // extra codes; Barcode is the main one
	PackCodes   []PackCode `json:"packCodes"`
}

type PackCode struct {
	PackID  int64  `json:"packId"`
	Barcode string `json:"barcode"`
}

type CopyCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CopyCustomer struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Balance     string `json:"balance"` // > 0 owed, < 0 advance
	CreditLimit string `json:"creditLimit"`
}

// CopyPerson is a cashier or manager who may sign in from the copy.
type CopyPerson struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Phone           string `json:"phone"`
	Role            string `json:"role"` // cashier | manager
	PinHash         string `json:"pinHash"`
	CanManageCredit bool   `json:"canManageCredit"`
	CanSeeCost      bool   `json:"canSeeCost"`
	// LastCount: their last close, note by note ({value: count}), for "Continue last count".
	LastCount map[string]float64 `json:"lastCount" db:"-"`
}

// CopyPlace is a locker or account a till may take a float from, bank into,
// or land a card payment in.
type CopyPlace struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	CashierAccess bool   `json:"cashierAccess"`
}

type CopySession struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"userId"`
	OpenedAt    string `json:"openedAt"`
	OpeningCash string `json:"openingCash"`
	Expected    string `json:"expected"` // cash the drawer should hold now
}

type CopyHeld struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID int64  `json:"customerId"` // 0 = walk-in
	Discount   string `json:"discount"`   // the bill discount
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
	CreatedAt  string `json:"createdAt"`
}
