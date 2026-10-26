package api

// ---- Setup → Settings (admin + manager) ----

// ShopSettings is every setting the Settings page edits. Appearance (System →
// Appearance) and the install id are not here. DefaultLockerID 0 = none.
type ShopSettings struct {
	ShopName               string `json:"shopName"`
	ShopNameLocal          string `json:"shopNameLocal"`
	Address                string `json:"address"`
	Phone                  string `json:"phone"`
	CurrencyCode           string `json:"currencyCode"`
	CurrencySymbol         string `json:"currencySymbol"`
	LogoURL                string `json:"logoUrl"`
	ReceiptFooter          string `json:"receiptFooter"`
	ReceiptWidth           string `json:"receiptWidth"`    // "80" | "58"
	DefaultSaleType        string `json:"defaultSaleType"` // "retail" | "wholesale"
	AskToPrint             bool   `json:"askToPrint"`
	PrintAs                string `json:"printAs"`      // "classic" | "image"
	LabelPrintAs           string `json:"labelPrintAs"` // "classic" | "image"
	TaxRegistered          bool   `json:"taxRegistered"`
	TaxRegNo               string `json:"taxRegNo"`
	OpenCashDrawer         bool   `json:"openCashDrawer"`
	DrawerKickPin          int    `json:"drawerKickPin"` // 0 = pin 2, 1 = pin 5
	AllowUntrackedCash     bool   `json:"allowUntrackedCash"`
	DefaultLockerID        int64  `json:"defaultLockerId"`
	AllowContinueLastCount bool   `json:"allowContinueLastCount"`
	LowStockAlerts         bool   `json:"lowStockAlerts"`
	StockTakeEnabled       bool   `json:"stockTakeEnabled"`
	ForcePinChange         bool   `json:"forcePinChange"`
	AllowCashierPinChange  bool   `json:"allowCashierPinChange"`
	LockTimeoutMinutes     int    `json:"lockTimeoutMinutes"`
	ReceiptPrinter         string `json:"receiptPrinter"` // a detected queue or tcp://host:9100; "" = system default
	LabelPrinter           string `json:"labelPrinter"`
	LabelWidthMM           int    `json:"labelWidthMm"`
	LabelHeightMM          int    `json:"labelHeightMm"`
	LabelGapMM             int    `json:"labelGapMm"`
	LabelNameLines         int    `json:"labelNameLines"`
	LabelBottomLines       int    `json:"labelBottomLines"`
	LabelNameFont          string `json:"labelNameFont"`      // "1".."4"
	LabelBottomFont        string `json:"labelBottomFont"`    // "1".."4"
	ReorderDefaultDays     int    `json:"reorderDefaultDays"` // supplier rhythm when nothing is known
	ReorderSafetyDays      int    `json:"reorderSafetyDays"`
}

// SettingsLocker is an active locker for the default-locker picker.
type SettingsLocker struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

// SettingsPage is GET /admin/settings.
type SettingsPage struct {
	Settings     ShopSettings     `json:"settings"`
	Logo         string           `json:"logo"`         // what prints: the uploaded image, else the URL; "" = none
	LogoUploaded bool             `json:"logoUploaded"` // an uploaded image (works offline) is set
	Printers     []string         `json:"printers"`     // queues detected on this computer; null in a save's answer (keep the list you have)
	Lockers      []SettingsLocker `json:"lockers"`
}
