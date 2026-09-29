package api

// ---- Admin: catalog (units, categories, products, services, groups) ----

type UnitRequest struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	AllowDecimal bool   `json:"allowDecimal"`
}
