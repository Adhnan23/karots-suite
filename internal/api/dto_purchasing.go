package api

// MoneySource is somewhere admin money can come from or go to: a locker (with
// its live balance) or a cashier's open till. Value is what requests send back
// ("locker:3", "till:5"); "" means outside the shop (card/online not tracked).
type MoneySource struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Group   string `json:"group"` // "Lockers" | "Tills"
	Kind    string `json:"kind"`  // locker kind (safe, bank, pocket, other, card) or "till"
	Balance string `json:"balance"`
}
