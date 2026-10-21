package api

import "time"

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
	ShopName   string `json:"shopName"`
	ServerTime string `json:"serverTime"` // RFC3339 UTC
	FileMode   bool   `json:"fileMode"`   // the shop runs its own PostgreSQL (one PC)
	Palette    string `json:"palette"`    // the shop's palette, so a device that never signed in shows it
	// The shop's active notes and coins (face values only), for the login / lock
	// easter-egg shop game's cash drawer.
	Notes []string `json:"notes"`
	Coins []string `json:"coins"`
	// The receipt header every customer already gets on paper (the shop game's receipt).
	Receipt ShopHeader `json:"receipt"`
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
	CanSeeLowStock     bool `json:"canSeeLowStock"` // the raw flag; MaySeeLowStock adds admin/manager
	CanExitKiosk       bool `json:"canExitKiosk"`
	IsSystem           bool `json:"isSystem"`
}

// Appearance is the system-user-locked look. BrandColor / Radius are the legacy
// look (before palettes owned brand and radius): read-only, applied while the
// palette is a built-in, cleared when another palette is chosen.
type Appearance struct {
	Palette      string `json:"palette"`      // "studio" | "night" | "bazaar" | "lagoon" | "catppuccin" | "custom-<id>"
	BrandColor   string `json:"brandColor"`   // legacy "#rrggbb" or ""
	Radius       string `json:"radius"`       // legacy "sharp" | "rounded" | "round"
	ReceiptStyle string `json:"receiptStyle"` // Image-mode style: "classic" | "bold" | "minimal" | "ticket" | "poster" | "grid" | "stamp" | "elegant" | "cards" | "sections"
}

// Palette is a custom palette (docs/specs/2026-10-21-palettes-design.md §3). CSS is
// the whole palette file; the list leaves it empty.
type Palette struct {
	ID        int64     `json:"id"`
	Theme     string    `json:"theme"`
	Name      string    `json:"name"`
	Base      string    `json:"base"`
	CSS       string    `json:"css"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PaletteInput struct {
	Name string `json:"name"`
	Base string `json:"base"`
	CSS  string `json:"css"`
}

type Me struct {
	User               SessionUser `json:"user"`
	Flags              Flags       `json:"flags"`
	Locked             bool        `json:"locked"`
	MustChangePin      bool        `json:"mustChangePin"`
	ShopName           string      `json:"shopName"`
	LockTimeoutMinutes int         `json:"lockTimeoutMinutes"`
	StockTakeEnabled   bool        `json:"stockTakeEnabled"` // gates Stock-take + Intake (karots-pos toggle)
	TaxRegistered      bool        `json:"taxRegistered"`    // shows the Tax report (karots-pos hides it otherwise)
	Appearance         Appearance  `json:"appearance"`
	Plugins            []string    `json:"plugins"`
}
