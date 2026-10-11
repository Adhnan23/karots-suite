package plugin

import (
	"context"
	"net/url"
)

// ProductFormValidate runs BEFORE a product is created/updated on the admin form.
// A plugin returns an apperr.Validation when a required custom field is missing;
// core aborts the save and shows the error. The client-side `required` attribute
// covers the everyday case — this is the server backstop.
type ProductFormValidate func(ctx context.Context, form url.Values) error

// ProductSaved runs AFTER a successful product create/update, with the raw form,
// so a plugin can persist its own fields (e.g. custom-field values). Errors are
// surfaced but the product is already saved.
type ProductSaved func(ctx context.Context, productID int64, form url.Values) error

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
	productValidators        hook[ProductFormValidate]
	productSavedHooks        hook[ProductSaved]
	productSearchers         hook[ProductSearchContributor]
	productMetaProviders     hook[ProductMetaProvider]
	productBadgeProviders    hook[ProductBadgeProvider]
	productSaleSuggesters    hook[ProductSaleSuggestionProvider]
	productDetailProviders   hook[ProductDetailContributor]
	productLabelProviders    hook[ProductLabelFieldProvider]
	productReorderAnnotators hook[ProductReorderAnnotator]
)

func (r *Registry) AddProductFormValidate(v ProductFormValidate) {
	productValidators.add(current, v)
}
func (r *Registry) AddProductSaved(s ProductSaved) { productSavedHooks.add(current, s) }
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

func ProductFormValidators() []ProductFormValidate          { return productValidators.all() }
func ProductSavedHooks() []ProductSaved                     { return productSavedHooks.all() }
func ProductSearchContributors() []ProductSearchContributor { return productSearchers.all() }
func ProductMetaProviders() []ProductMetaProvider           { return productMetaProviders.all() }
func ProductBadgeProviders() []ProductBadgeProvider         { return productBadgeProviders.all() }
func ProductSaleSuggestionProviders() []ProductSaleSuggestionProvider {
	return productSaleSuggesters.all()
}
func ProductDetailContributors() []ProductDetailContributor   { return productDetailProviders.all() }
func ProductLabelFieldProviders() []ProductLabelFieldProvider { return productLabelProviders.all() }
func ProductReorderAnnotators() []ProductReorderAnnotator     { return productReorderAnnotators.all() }
