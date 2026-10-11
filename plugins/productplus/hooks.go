package productplus

import (
	"context"
	"encoding/json"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"
)

// The core hooks Product Plus fills: the product form / intake extras, products
// sheet columns, search, the list line, till ⓘ rows and label lines.
func (p *Plugin) registerHooks(reg *plugin.Registry) {
	reg.AddProductExtras(plugin.ProductExtras{Key: "productplus", Validate: p.validateExtra, Save: p.saveExtra})
	reg.AddProductSheetColumns(plugin.ProductSheetColumns{Key: "productplus", Label: "Custom fields",
		Columns: p.store.SheetColumns, Export: p.store.SheetExport, Import: p.store.SheetImport})
	reg.AddProductSearchContributor(plugin.ProductSearchContributor{Match: p.store.MatchProductIDs})
	reg.AddProductMetaProvider(plugin.ProductMetaProvider{Batch: p.store.MetaFor})
	reg.AddProductDetailContributor(plugin.ProductDetailContributor{Rows: p.store.TillRows})
	reg.AddProductLabelFieldProvider(plugin.ProductLabelFieldProvider{Rows: p.store.LabelRows})
}

func decodeExtra(raw json.RawMessage) (map[string]string, error) {
	var b extraBody
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, apperr.BadRequest("invalid custom fields")
	}
	return b.Values, nil
}

// validateExtra checks the values the form sent (only those; a field it didn't
// show stays as it is) before core writes the product.
func (p *Plugin) validateExtra(ctx context.Context, x plugin.ExtraCtx, raw json.RawMessage) error {
	vals, err := decodeExtra(raw)
	if err != nil {
		return err
	}
	return p.store.CheckProduct(ctx, x.ProductID, x.CategoryID, "form", vals)
}

func (p *Plugin) saveExtra(ctx context.Context, x plugin.ExtraCtx, raw json.RawMessage) error {
	vals, err := decodeExtra(raw)
	if err != nil {
		return err
	}
	return p.store.SaveProduct(ctx, x.ProductID, x.CategoryID, "form", vals)
}
