package api

import "time"

// SessionUser is who is signed in.
type SessionUser struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // "admin" | "manager" | "cashier"
	// SendsToCashier: a sales person (a cashier who prepares carts, no money).
	SendsToCashier bool `json:"sendsToCashier"`
}

type LoginRequest struct {
	Phone string `json:"phone"`
	Pin   string `json:"pin"`
	// DeviceLabel names this device for one person one device ("Phone 2");
	// "" = named from its browser.
	DeviceLabel string `json:"deviceLabel,omitempty"`
	// App is "shelf" for Karots Shelf: only a person who may use it signs in,
	// and the sign-in never takes their one device (a till stays where it is).
	App string `json:"app,omitempty"`
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
	MayUseShelf        bool `json:"mayUseShelf"` // Karots Shelf: admin / manager, or the tick
}

// Appearance is the system-user-locked look. BrandColor / Radius are the legacy
// look (before palettes owned brand and radius): read-only, applied while the
// palette is a built-in, cleared when another palette is chosen.
type Appearance struct {
	Palette      string `json:"palette"`      // a built-in id (settings.Skins) | "custom-<id>"
	BrandColor   string `json:"brandColor"`   // legacy "#rrggbb" or ""
	Radius       string `json:"radius"`       // legacy "sharp" | "rounded" | "round"
	ReceiptStyle string `json:"receiptStyle"` // Image-mode style: "classic" | "bold" | "minimal" | "ticket" | "poster" | "grid" | "stamp" | "elegant" | "cards" | "sections" | "compact"
	// ReceiptTextSize is the slips' text size in percent: 100 | 115 | 130. On a save, 0 (absent) changes nothing.
	ReceiptTextSize int `json:"receiptTextSize,omitempty" tstype:",optional"`
	// LabelStyles is the Image-mode style per label kind ("price", plugin kinds). On a
	// save it is merged: a kind set to "" goes back to classic, an absent map changes nothing.
	LabelStyles map[string]string `json:"labelStyles,omitempty" tstype:",optional"`
}

// Palette is a custom palette. CSS is
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
	User SessionUser `json:"user"`
	// MovedTo is the device the person moved to ("" = they are on this one).
	MovedTo            string     `json:"movedTo"`
	Flags              Flags      `json:"flags"`
	Locked             bool       `json:"locked"`
	MustChangePin      bool       `json:"mustChangePin"`
	ShopName           string     `json:"shopName"`
	LockTimeoutMinutes int        `json:"lockTimeoutMinutes"`
	StockTakeEnabled   bool       `json:"stockTakeEnabled"` // gates Stock-take + Intake
	TaxRegistered      bool       `json:"taxRegistered"`    // shows the Tax report
	Appearance         Appearance `json:"appearance"`
	Plugins            []string   `json:"plugins"`
}

// SellLayout is the Sell page's look on one screen size (Appearance, system user):
// Arr = arrangement id (P products, C cart, Y pay; "_" = next column), Fr = each
// column's share of the width, Split = the top part's share of a stacked column,
// Card = "L" | "M" | "S" | "T" (list), Text = 90 | 100 | 110 (percent).
type SellLayout struct {
	Arr   string    `json:"arr"`
	Fr    []float64 `json:"fr"`
	Split float64   `json:"split"`
	Card  string    `json:"card"`
	Text  int       `json:"text"`
}

// ScreenSize is a size a till showed the Sell page at (CSS px), and who used it.
type ScreenSize struct {
	W        int      `json:"w"`
	H        int      `json:"h"`
	Users    []string `json:"users"`
	LastSeen string   `json:"lastSeen"`
}

// SellLayoutPage is Appearance → Sell page layout: the saved layouts by "<w>x<h>"
// and the sizes seen in the last 90 days, most recent first.
type SellLayoutPage struct {
	Layouts map[string]SellLayout `json:"layouts"`
	Screens []ScreenSize          `json:"screens"`
}

// SellLayoutSave replaces every saved layout (a size left out goes back to today's).
type SellLayoutSave struct {
	Layouts map[string]SellLayout `json:"layouts"`
}
