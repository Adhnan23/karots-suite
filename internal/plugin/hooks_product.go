package plugin

import (
	"context"
	"encoding/json"
)

// ExtraCtx is the product a plugin's extras belong to. ProductID is 0 while
// validating a new product; IsNew is true on create (admin form and intake).
type ExtraCtx struct {
	ProductID  int64
	CategoryID int64
	IsNew      bool
}

// ProductExtras lets a plugin carry its own data on the product form and intake
// (ProductRequest.extra[Key], opaque to core). Validate runs for every key sent
// BEFORE the product is written — an error refuses the save and nothing is
// written; Save runs AFTER, with the product's id. A key no plugin owns is a 400.
type ProductExtras struct {
	Key      string
	Validate func(ctx context.Context, x ExtraCtx, raw json.RawMessage) error
	Save     func(ctx context.Context, x ExtraCtx, raw json.RawMessage) error
}

// ProductSheetColumns lets a plugin add its own columns to the products sheet
// (export, template, import) when the admin ticks its toggle (Label) on
// Products → Sheets. Columns names them (the header); Export returns each
// product's cells in that order (omit an id for blanks); Import gets one row's
// non-blank cells keyed by column, after core saved the product, and returns a
// note for the import report (a bad cell is a note, not a failed row).
type ProductSheetColumns struct {
	Key, Label string
	Columns    func(ctx context.Context) ([]string, error)
	Export     func(ctx context.Context, productIDs []int64) (map[int64][]string, error)
	Import     func(ctx context.Context, productID, categoryID int64, cells map[string]string) (note string, err error)
}

// ProductSearchContributor lets a plugin add product ids to a search. Match
// returns the ids whose plugin-owned, searchable data matches the query; the web
// layer folds them into products.SearchContributor (which products.List ORs into
// its search). Keep it one cheap query; return (nil, err) defensively.
type ProductSearchContributor struct {
	Match func(ctx context.Context, query string) ([]int64, error)
}

// ProductMetaProvider supplies read-only display text for products (e.g. a
// plugin's custom-field values) shown in the admin product list. Batch is called
// once per rendered page with every visible product id and returns a short display
// string per id (omit an id for nothing to show). Best-effort: an error just hides
// the meta, never breaks the list. One query for the whole page — no per-row calls.
type ProductMetaProvider struct {
	Batch func(ctx context.Context, productIDs []int64) (map[int64]string, error)
}

// ProductBadgeProvider supplies short badge labels for products, shown as pins on
// the till product card. Batch is called with the visible product ids and returns
// each id's badges (omit an id for none). Best-effort: an error just drops the
// badges. One query per page — no per-row calls.
type ProductBadgeProvider struct {
	Batch func(ctx context.Context, productIDs []int64) (map[int64][]string, error)
}

// SaleSuggestion is an optional line discount a plugin proposes when a product
// is added at the till (e.g. a markdown on slow stock). The cashier is prompted to
// apply or skip it; applying sets the line's existing discount fields.
type SaleSuggestion struct {
	DiscountType  string // "percent" | "fixed" (matches sale_items.discount_type)
	DiscountValue string // percent (e.g. "20") or fixed per-unit amount
	Label         string // short badge/summary, e.g. "Markdown -20%"
	Prompt        string // popup body, e.g. "Slow item — apply 20% off?"
}

// ProductSaleSuggestionProvider supplies a line-discount suggestion per product
// for the till. Batch is called with the visible product ids and returns each
// id's suggestion (omit an id for none). Best-effort: an error drops the
// suggestion, never breaks the payload. One query per page — no per-row calls.
type ProductSaleSuggestionProvider struct {
	Batch func(ctx context.Context, productIDs []int64) (map[int64]SaleSuggestion, error)
}

// DetailRow is one labeled line a plugin contributes to the till product-info
// popup.
type DetailRow struct {
	Label string
	Value string
}

// ProductDetailContributor supplies extra labeled rows for the till product-info
// popup (e.g. a plugin's custom fields marked "show at till"). Rows is called for
// the single product whose info the cashier opened. Best-effort: an error just
// drops the rows, never blocks the popup.
type ProductDetailContributor struct {
	Rows func(ctx context.Context, productID int64) ([]DetailRow, error)
}

// ProductLabelFieldProvider supplies a product's custom fields flagged "print on
// label" — the selectable options for the barcode label's top/bottom line.
// Best-effort: an error just yields no extra options.
type ProductLabelFieldProvider struct {
	Rows func(ctx context.Context, productID int64) ([]DetailRow, error)
}

// ReorderNote annotates one low-stock product on the reorder worklist. Covered
// means an interchangeable equivalent is in stock, so the item need not be
// reordered even though it reads low; Note is a short explanation shown on the row.
type ReorderNote struct {
	Covered bool
	Note    string
}

// ProductReorderAnnotator lets a plugin annotate the low-stock/reorder page: given
// the visible low-stock product ids, it returns per product whether an equivalent
// covers it and a short note. Batched, best-effort (an error drops the annotations).
//
// LowMembers (optional) returns product ids that must be PULLED INTO the worklist
// when the "filter by alternatives" toggle is on — e.g. every member of a tier that
// is low as a whole, so the whole low group is orderable even when an individual
// member isn't below its own reorder level. nil = contribute nothing extra.
type ProductReorderAnnotator struct {
	Batch      func(ctx context.Context, productIDs []int64) (map[int64]ReorderNote, error)
	LowMembers func(ctx context.Context) ([]int64, error)
}

var (
	productExtras            hook[ProductExtras]
	productSheetColumns      hook[ProductSheetColumns]
	productSearchers         hook[ProductSearchContributor]
	productMetaProviders     hook[ProductMetaProvider]
	productBadgeProviders    hook[ProductBadgeProvider]
	productSaleSuggesters    hook[ProductSaleSuggestionProvider]
	productDetailProviders   hook[ProductDetailContributor]
	productLabelProviders    hook[ProductLabelFieldProvider]
	productReorderAnnotators hook[ProductReorderAnnotator]
)

func (r *Registry) AddProductExtras(x ProductExtras) { productExtras.add(current, x) }
func (r *Registry) AddProductSheetColumns(x ProductSheetColumns) {
	productSheetColumns.add(current, x)
}
func (r *Registry) AddProductSearchContributor(s ProductSearchContributor) {
	productSearchers.add(current, s)
}
func (r *Registry) AddProductMetaProvider(p ProductMetaProvider) {
	productMetaProviders.add(current, p)
}
func (r *Registry) AddProductBadgeProvider(p ProductBadgeProvider) {
	productBadgeProviders.add(current, p)
}
func (r *Registry) AddProductSaleSuggestionProvider(p ProductSaleSuggestionProvider) {
	productSaleSuggesters.add(current, p)
}
func (r *Registry) AddProductDetailContributor(p ProductDetailContributor) {
	productDetailProviders.add(current, p)
}
func (r *Registry) AddProductLabelFieldProvider(p ProductLabelFieldProvider) {
	productLabelProviders.add(current, p)
}
func (r *Registry) AddProductReorderAnnotator(a ProductReorderAnnotator) {
	productReorderAnnotators.add(current, a)
}

func ProductExtrasHooks() []ProductExtras                   { return productExtras.all() }
func ProductSheetColumnsHooks() []ProductSheetColumns       { return productSheetColumns.all() }
func ProductSearchContributors() []ProductSearchContributor { return productSearchers.all() }
func ProductMetaProviders() []ProductMetaProvider           { return productMetaProviders.all() }
func ProductBadgeProviders() []ProductBadgeProvider         { return productBadgeProviders.all() }
func ProductSaleSuggestionProviders() []ProductSaleSuggestionProvider {
	return productSaleSuggesters.all()
}
func ProductDetailContributors() []ProductDetailContributor   { return productDetailProviders.all() }
func ProductLabelFieldProviders() []ProductLabelFieldProvider { return productLabelProviders.all() }
func ProductReorderAnnotators() []ProductReorderAnnotator     { return productReorderAnnotators.all() }
