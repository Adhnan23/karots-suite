package api

// SessionUser is who is signed in.
type SessionUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // "admin" | "manager" | "cashier"
}

type LoginRequest struct {
	Phone string `json:"phone"`
	Pin   string `json:"pin"`
}

type LoginResponse struct {
	User SessionUser `json:"user"`
}

// LoginInfo feeds the login page's hidden Konami reveal + console line.
type LoginInfo struct {
	InstallID  string `json:"installId"`
	ServerTime string `json:"serverTime"` // RFC3339 UTC
}

type ChangePinRequest struct {
	CurrentPin string `json:"currentPin"`
	NewPin     string `json:"newPin"`
	ConfirmPin string `json:"confirmPin"`
}

type Flags struct {
	CanHandleSuppliers bool `json:"canHandleSuppliers"`
	CanManageCredit    bool `json:"canManageCredit"`
	CanSeeCost         bool `json:"canSeeCost"`
	CanExitKiosk       bool `json:"canExitKiosk"`
	IsSystem           bool `json:"isSystem"`
}

// Appearance is the system-user-locked look. BrandColor "" = theme default.
type Appearance struct {
	BrandColor   string `json:"brandColor"`   // "#rrggbb" or ""
	Radius       string `json:"radius"`       // "sharp" | "rounded" | "round"
	Density      string `json:"density"`      // "comfortable" | "compact"
	ReceiptStyle string `json:"receiptStyle"` // "classic" | "modern" | "boxed" | "compact"
}

type Me struct {
	User               SessionUser `json:"user"`
	Flags              Flags       `json:"flags"`
	Locked             bool        `json:"locked"`
	MustChangePin      bool        `json:"mustChangePin"`
	ShopName           string      `json:"shopName"`
	LockTimeoutMinutes int         `json:"lockTimeoutMinutes"`
	Appearance         Appearance  `json:"appearance"`
	Plugins            []string    `json:"plugins"`
}
