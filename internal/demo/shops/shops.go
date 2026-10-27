// Package shops describes the Demo Room's shops: who works there, which plugins
// they run, how they look and what they sell.
package shops

// Login is a person the shop knows. Shown logins appear in the demo chrome so the
// owner never fumbles a PIN; staff logins belong to the alive simulator.
type Login struct {
	Role  string `json:"role"` // admin | manager | cashier
	Name  string `json:"name"`
	Phone string `json:"phone"` // the login form signs in by phone + PIN
	PIN   string `json:"pin"`
}

// Profile is one demo shop.
type Profile struct {
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	Blurb        string            `json:"blurb"`
	Plugins      []string          `json:"plugins"` // plugin keys; empty = core only
	Palette      string            `json:"palette"`
	ReceiptStyle string            `json:"receiptStyle"`
	LabelStyles  map[string]string `json:"labelStyles"` // label kind → style key
	Catalogue    string            `json:"catalogue"`   // grocery | pharmacy | phone | hardware | mixed
	Logins       []Login           `json:"logins"`
	Staff        []Login           `json:"-"`
	Volume       int               `json:"-"` // average sales per day
}

// ByKey finds a built-in shop.
func ByKey(key string) (Profile, bool) {
	for _, p := range BuiltIn() {
		if p.Key == key {
			return p, true
		}
	}
	return Profile{}, false
}
