// Package products is the master catalog: CRUD plus barcode/search lookups used
// by both the admin panel and the cashier terminal.
package products

import (
	"context"
)

// SearchContributor, when non-nil, contributes extra product ids to every search
// (List/Count) — used by a plugin to make its own searchable fields findable in
// core product search. It is a package-level func var, not a plugin import, so
// internal/features/products never depends on internal/plugin (which imports it):
// the web layer sets this during plugin setup. nil = core-only behaviour.
var SearchContributor func(ctx context.Context, query string) ([]int64, error)

// BadgeProvider, when non-nil, returns per-product badge labels for the till card
// (set by the web layer from plugin badge providers). nil = no badges.
var BadgeProvider func(ctx context.Context, productIDs []int64) map[int64][]string

// SaleSuggestion mirrors plugin.SaleSuggestion (products must not import plugin;
// the web layer converts and wires SaleSuggestionProvider from plugin hooks).
type SaleSuggestion struct {
	DiscountType  string `json:"discount_type"`
	DiscountValue string `json:"discount_value"`
	Label         string `json:"label"`
	Prompt        string `json:"prompt"`
}

// SaleSuggestionProvider, when non-nil, returns a per-product till discount
// suggestion (set by the web layer from plugin providers). nil = none.
var SaleSuggestionProvider func(ctx context.Context, productIDs []int64) map[int64]SaleSuggestion

// applySuggestions stamps each row's Suggestion from the provider (no-op when
// unset — a core-only build). Best-effort: skips silently on a nil map.
func applySuggestions(ctx context.Context, rows []Product) {
	if SaleSuggestionProvider == nil || len(rows) == 0 {
		return
	}
	ids := make([]int64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	m := SaleSuggestionProvider(ctx, ids)
	if m == nil {
		return
	}
	for i := range rows {
		if s, ok := m[rows[i].ID]; ok {
			sc := s
			rows[i].Suggestion = &sc
		}
	}
}

// DetailRow is one labeled line in the till product-info popup.
type DetailRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DetailContributor, when non-nil, returns extra labeled rows for the till
// product-info popup (a plugin's custom fields marked "show at till"). Set by the
// web layer from plugin detail contributors; nil = core rows only.
var DetailContributor func(ctx context.Context, productID int64) []DetailRow

// LabelFieldProvider, when non-nil, returns a product's custom fields flagged
// "print on label" — the selectable options for the barcode label top/bottom
// line. Set by the web layer from plugin label-field providers; nil = none.
var LabelFieldProvider func(ctx context.Context, productID int64) []DetailRow

// Decorate fills the plugin-contributed, non-persisted fields (till badges and
// sale suggestions) on rows, exactly as the legacy List/Get handlers did. The
// JSON till layer calls it for every product list it returns.
func Decorate(ctx context.Context, rows []Product) {
	if BadgeProvider != nil && len(rows) > 0 {
		ids := make([]int64, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		if m := BadgeProvider(ctx, ids); m != nil {
			for i := range rows {
				rows[i].Badges = m[rows[i].ID]
			}
		}
	}
	applySuggestions(ctx, rows)
}
