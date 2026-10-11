// Package products is the master catalog: CRUD plus barcode/search lookups used
// by both the admin panel and the cashier terminal.
package products

import (
	"context"
	"time"

	"karots-suite/internal/db"

	"github.com/lib/pq"

	"github.com/shopspring/decimal"
)

type Product struct {
	ID                  int64           `db:"id"              json:"id"`
	Name                string          `db:"name"            json:"name"`
	NameLocal           *string         `db:"name_local"      json:"name_local,omitempty"`
	Barcode             *string         `db:"barcode"         json:"barcode,omitempty"`
	CategoryID          int64           `db:"category_id"     json:"category_id"`
	UnitID              int64           `db:"unit_id"         json:"unit_id"`
	CostPrice           decimal.Decimal `db:"cost_price"      json:"cost_price"`
	SellingPrice        decimal.Decimal `db:"selling_price"   json:"selling_price"`
	WholesalePrice      decimal.Decimal `db:"wholesale_price" json:"wholesale_price"`
	TaxRate             decimal.Decimal `db:"tax_rate"        json:"tax_rate"`
	ReorderLevel        int             `db:"reorder_level"   json:"reorder_level"`
	HasExpiry           bool            `db:"has_expiry"      json:"has_expiry"`
	TrackSerial         bool            `db:"track_serial"    json:"track_serial"`
	WarrantyMonths      int             `db:"warranty_months"       json:"warranty_months"`
	IsActive            bool            `db:"is_active"             json:"is_active"`
	IsService           bool            `db:"is_service"            json:"is_service"`
	PassThrough         bool            `db:"pass_through"          json:"pass_through"`
	NeedsReview         bool            `db:"needs_review"          json:"needs_review"`
	PreferredSupplierID *int64          `db:"preferred_supplier_id" json:"preferred_supplier_id,omitempty"`
	CreatedBy           *int64          `db:"created_by"            json:"created_by,omitempty"`
	CreatedAt           time.Time       `db:"created_at"            json:"created_at"`
	// Joined, read-only:
	CategoryName          string          `db:"category_name"           json:"category_name"`
	UnitAbbr              string          `db:"unit_abbr"               json:"unit_abbr"`
	UnitAllowDecimal      bool            `db:"unit_allow_decimal"      json:"unit_allow_decimal"`
	StockQty              decimal.Decimal `db:"stock_qty"               json:"stock_qty"`
	CreatedByName         *string         `db:"created_by_name"         json:"created_by_name,omitempty"`
	PreferredSupplierName *string         `db:"preferred_supplier_name" json:"preferred_supplier_name,omitempty"`
	// Plugin-contributed, not persisted: till-card badge pins (e.g. quality tier).
	Badges []string `db:"-" json:"badges,omitempty"`
	// Plugin-contributed, not persisted: an optional till line-discount suggestion
	// (e.g. a plugin markdown) the cashier is prompted to apply.
	Suggestion *SaleSuggestion `db:"-" json:"suggestion,omitempty"`
}

// IsLowStock reports whether on-hand quantity is at or below the reorder level.
func (p Product) IsLowStock() bool {
	return p.StockQty.LessThanOrEqual(decimal.NewFromInt(int64(p.ReorderLevel)))
}

type CreateInput struct {
	Name           string  `json:"name"            form:"name"            validate:"required,min=1,max=150"`
	NameLocal      *string `json:"name_local"      form:"name_local"`
	Barcode        *string `json:"barcode"         form:"barcode"         validate:"omitempty,max=50"`
	CategoryID     int64   `json:"category_id"     form:"category_id"     validate:"required,gt=0"`
	UnitID         int64   `json:"unit_id"         form:"unit_id"         validate:"required,gt=0"`
	CostPrice      string  `json:"cost_price"      form:"cost_price"`
	SellingPrice   string  `json:"selling_price"   form:"selling_price"`
	WholesalePrice string  `json:"wholesale_price" form:"wholesale_price"`
	TaxRate        string  `json:"tax_rate"        form:"tax_rate"`
	ReorderLevel   int     `json:"reorder_level"   form:"reorder_level"   validate:"gte=0"`
	TrackSerial    bool    `json:"track_serial"    form:"track_serial"`
	WarrantyMonths int     `json:"warranty_months" form:"warranty_months" validate:"gte=0,lte=600"`
	// IsService marks a non-stocked service line. Not on the normal admin form
	// (defaults false); set by plugins via the products service.
	IsService bool `json:"is_service" form:"is_service"`
	// PreferredSupplierID is an optional default supplier (nil/0 = none). The form
	// posts an empty string for "none"; bind to *int64 so 0 stays nil.
	PreferredSupplierID *int64 `json:"preferred_supplier_id" form:"preferred_supplier_id"`
	// RepriceStock applies a changed selling price to the stock already on the
	// shelf, by clearing the per-lot prices so those lots follow the product
	// again. Only meaningful on update; a new product has no lots yet.
	RepriceStock bool `json:"reprice_stock" form:"reprice_stock"`
}

type UpdateInput = CreateInput

type ListQuery struct {
	Page       int    `query:"page"        form:"page"`
	Limit      int    `query:"limit"       form:"limit"`
	CategoryID *int64 `query:"category_id" form:"category_id"`
	Search     string `query:"search"      form:"search"`
	LowStock   bool   `query:"low_stock"   form:"low_stock"`
	// PreferredSupplierID filters to products whose preferred (default) supplier is
	// this one — used by the reorder picker to build a per-supplier order. nil = all.
	PreferredSupplierID *int64 `query:"supplier_id" form:"supplier_id"`
	// IncludeServices lets a caller see non-stocked service lines, which every
	// other list deliberately hides (stock, valuation and low-stock columns are
	// meaningless for them). Needed by pickers that attach a service to
	// something — a cashier menu card, for instance — otherwise a service can be
	// created but never put in front of a cashier.
	IncludeServices bool `query:"include_services" form:"include_services"`
	// IncludeInactive shows disabled products too (the admin "Show disabled"
	// toggle). Off by default, so every existing caller keeps seeing active only.
	IncludeInactive bool `query:"include_inactive" form:"include_inactive"`
	// Fuzzy enables typo-tolerant matching. Not a query param: the service sets
	// it only for the rescue pass after a strict search came back empty.
	Fuzzy bool `query:"-" form:"-"`
}

// MaxListLimit is the largest page List will serve. Callers that need the whole
// catalog must use ListAll — List is for paging, not for bulk reads.
const MaxListLimit = 100

// Normalize applies sane pagination defaults/bounds.
//
// Over-asking clamps DOWN TO THE MAXIMUM, never to something smaller. The old
// rule was "> 100 → 50", so a caller asking for 10000 silently got 50 — fewer
// than the cap it exceeded. That turned every bulk read into a quiet 50-row
// truncation, and any total summed from those rows was wrong (the Inventory
// Valuation report reported 14% of real stock value this way).
func (q *ListQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	switch {
	case q.Limit < 1:
		q.Limit = 50
	case q.Limit > MaxListLimit:
		q.Limit = MaxListLimit
	}
}

func (q ListQuery) offset() int { return (q.Page - 1) * q.Limit }

// --- repository ---

const selectProduct = `
	SELECT p.*, c.name AS category_name, u.abbreviation AS unit_abbr,
	       u.allow_decimal AS unit_allow_decimal,
	       COALESCE(s.quantity, 0) AS stock_qty,
	       psup.name AS preferred_supplier_name
	FROM products p
	JOIN categories c ON c.id = p.category_id
	JOIN units u      ON u.id = p.unit_id
	LEFT JOIN stock s ON s.product_id = p.id
	LEFT JOIN suppliers psup ON psup.id = p.preferred_supplier_id`

type Repository struct{ db db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{db: q} }

// searchMatchIDs returns the plugin-contributed ids for a query (empty when no
// contributor is set, no search term, or the contributor errors — it must never
// break the core search).
func searchMatchIDs(ctx context.Context, q ListQuery) []int64 {
	if q.Search == "" || SearchContributor == nil {
		return nil
	}
	ids, err := SearchContributor(ctx, q.Search)
	if err != nil {
		return nil
	}
	return ids
}

// subcatsCTE expands a selected category to itself + all descendants so that
// filtering by a parent category also returns products in its sub-categories.
// $4 is the category id; the search placeholders occupy $1-$3 (see searchClause).
const subcatsCTE = `
	WITH RECURSIVE subcats AS (
		SELECT id FROM categories WHERE $4::bigint IS NOT NULL AND id = $4
		UNION ALL
		SELECT c.id FROM categories c JOIN subcats sc ON c.parent_id = sc.id
	)`

// searchArgs binds the three searchClause placeholders for a query.
func (q ListQuery) searchArgs() (any, any, bool) {
	toks := searchTokens(q.Search)
	if len(toks) == 0 {
		return nil, nil, false
	}
	return pq.Array(toks), q.Search, q.Fuzzy
}

func (r *Repository) List(ctx context.Context, q ListQuery) ([]Product, error) {
	toks, raw, fuzzy := q.searchArgs()
	var rows []Product
	err := r.db.SelectContext(ctx, &rows, subcatsCTE+selectProduct+`
		WHERE (p.is_active = true OR $9) AND (p.is_service = false OR $8)
		  AND (`+searchClause+` OR p.id = ANY($11::bigint[]))
		  AND ($4::bigint IS NULL OR p.category_id IN (SELECT id FROM subcats))
		  AND ($5 = false OR COALESCE(s.quantity,0) <= p.reorder_level)
		  AND ($10::bigint IS NULL OR p.preferred_supplier_id = $10)
		ORDER BY `+searchRank+` p.name, p.id
		LIMIT $6 OFFSET $7`,
		toks, raw, fuzzy, q.CategoryID, q.LowStock, q.Limit, q.offset(), q.IncludeServices, q.IncludeInactive, q.PreferredSupplierID,
		pq.Array(searchMatchIDs(ctx, q)))
	return rows, err
}

// FrequentProducts backs the till's dynamic "Frequently sold" shortcut grid on
// the top menu: full product cards ranked by net quantity sold (after returns)
// since `since`. It excludes services, pass-through products (reload face value
// can't be tap-added), disabled products, and anything currently out of stock —
// so the grid only ever offers real, sellable stock a cashier can tap.
func (r *Repository) FrequentProducts(ctx context.Context, since time.Time, limit int) ([]Product, error) {
	var ids []int64
	if err := r.db.SelectContext(ctx, &ids, `
		SELECT si.product_id
		FROM sale_items si
		JOIN sales s    ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		LEFT JOIN stock st ON st.product_id = p.id
		WHERE s.status <> 'void' AND s.created_at >= $1
		  AND p.is_active AND NOT p.is_service AND NOT p.pass_through
		  AND COALESCE(st.quantity, 0) > 0
		GROUP BY si.product_id
		HAVING SUM(si.quantity - si.returned_qty) > 0
		ORDER BY SUM(si.quantity - si.returned_qty) DESC, MAX(s.created_at) DESC
		LIMIT $2`, since, limit); err != nil {
		return nil, err
	}
	return r.byIDsOrdered(ctx, ids)
}

// byIDsOrdered loads full product cards for the given ids and returns them in the
// same order as `ids` (a single SELECT ... = ANY, re-sorted in Go — Postgres does
// not preserve IN-list order).
func (r *Repository) byIDsOrdered(ctx context.Context, ids []int64) ([]Product, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []Product
	if err := r.db.SelectContext(ctx, &rows, selectProduct+` WHERE p.id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	byID := make(map[int64]Product, len(rows))
	for _, p := range rows {
		byID[p.ID] = p
	}
	out := make([]Product, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// searchRank puts the most likely hit first when the user is searching: an exact
// barcode, then names that START with what was typed (so "pen" leads with "Pen
// Box", not "Atlas Chooty Blue Pen"), then everything else alphabetically. With
// no search term every product ranks 3 and the order collapses to plain by-name.
const searchRank = `
	CASE
		WHEN $2::text IS NOT NULL AND p.barcode = $2 THEN 0
		WHEN $2::text IS NOT NULL AND lower(p.name) LIKE lower($2) || '%' THEN 1
		WHEN $2::text IS NOT NULL AND lower(p.name) LIKE '%' || lower($2) || '%' THEN 2
		ELSE 3
	END,`

// ListAll returns every active, non-service product with no pagination, for
// spreadsheet export where the whole catalog must round-trip (the paginated
// List clamps Limit to 100, so it can never stream more than one page).
func (r *Repository) ListAll(ctx context.Context) ([]Product, error) {
	var rows []Product
	err := r.db.SelectContext(ctx, &rows, selectProduct+`
		WHERE p.is_active = true AND p.is_service = false
		ORDER BY p.name, p.id`)
	return rows, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Product, error) {
	var p Product
	err := r.db.GetContext(ctx, &p, selectProduct+` WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) FindByBarcode(ctx context.Context, barcode string) (*Product, error) {
	var p Product
	err := r.db.GetContext(ctx, &p,
		selectProduct+` WHERE p.barcode = $1 AND p.is_active = true`, barcode)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByName matches an active product by case-insensitive name. Used by the
// catalog import to make the export→edit→re-upload round-trip idempotent for
// barcode-less products (which can't be matched on barcode). When several share
// a name it returns the oldest, so repeated imports converge on one row.
func (r *Repository) FindByName(ctx context.Context, name string) (*Product, error) {
	var p Product
	err := r.db.GetContext(ctx, &p,
		selectProduct+` WHERE LOWER(p.name) = LOWER($1) AND p.is_active = true ORDER BY p.id LIMIT 1`, name)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// BarcodeExists reports whether any product (active or not) already carries this
// barcode, so a generated code never shadows an existing or deactivated product.
func (r *Repository) BarcodeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists,
		`SELECT EXISTS(SELECT 1 FROM products WHERE barcode = $1)`, code)
	return exists, err
}

// ListServices returns active service products. Services are few by nature
// (a print counter, a coffee, a top-up line), so this is an unpaged list
// meant for pickers that attribute something to a service.
// AllServices is every service product, disabled ones last, for the admin list
// (where a disabled service must stay reachable to be enabled again).
func (r *Repository) AllServices(ctx context.Context) ([]Product, error) {
	var rows []Product
	err := r.db.SelectContext(ctx, &rows, selectProduct+`
		WHERE p.is_service
		ORDER BY p.is_active DESC, p.name`)
	return rows, err
}

func (r *Repository) ListServices(ctx context.Context) ([]Product, error) {
	var rows []Product
	err := r.db.SelectContext(ctx, &rows, selectProduct+`
		WHERE p.is_service AND p.is_active
		ORDER BY p.name`)
	return rows, err
}
