package api

// ---- Admin: catalog (units, categories, products, services, groups) ----

type UnitRequest struct {
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	AllowDecimal bool   `json:"allowDecimal"`
}

// CategoryNode is one row of the depth-first category tree. ParentID 0 = top level;
// Path holds ancestor ids, root first (not self).
type CategoryNode struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	ParentID     int64   `json:"parentId"`
	Depth        int     `json:"depth"`
	ChildCount   int     `json:"childCount"`
	Path         []int64 `json:"path"`
	ProductCount int     `json:"productCount"` // products filed directly here
}

// CategoryRequest creates or edits a category. ParentID 0 = top level.
type CategoryRequest struct {
	Name     string `json:"name"`
	ParentID int64  `json:"parentId"`
}

// CategoryQuickResult answers the pickers' inline create (find-or-create under a parent).
type CategoryQuickResult struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Depth int    `json:"depth"`
}
