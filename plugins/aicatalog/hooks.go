package aicatalog

import (
	"context"
	"encoding/json"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"
)

// extraBody is what ✨ Identify puts in ProductRequest.extra["aicatalog"] on a
// pick; it is learnt when the product is saved.
type extraBody struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Specs       string `json:"specs"`
	Explanation string `json:"explanation"`
	Source      string `json:"source"` // ai_confident | user_picked | user_explained
	Query       string `json:"query"`
}

var sources = map[string]bool{"ai_confident": true, "user_picked": true, "user_explained": true}

func decodeExtra(raw json.RawMessage) (extraBody, error) {
	var b extraBody
	if json.Unmarshal(raw, &b) != nil || strings.TrimSpace(b.Name) == "" || !sources[b.Source] {
		return b, apperr.BadRequest("invalid AI pick")
	}
	return b, nil
}

func (p *Plugin) registerHooks(reg *plugin.Registry) {
	reg.AddProductMerger(plugin.ProductMerger{Key: "aicatalog", Merge: mergeProduct})
	reg.AddProductExtras(plugin.ProductExtras{Key: "aicatalog",
		Validate: func(_ context.Context, _ plugin.ExtraCtx, raw json.RawMessage) error {
			_, err := decodeExtra(raw)
			return err
		},
		Save: p.learn,
	})
	reg.AddProductDetailContributor(plugin.ProductDetailContributor{Rows: p.detailRows})
}

func (p *Plugin) learn(ctx context.Context, x plugin.ExtraCtx, raw json.RawMessage) error {
	b, err := decodeExtra(raw)
	if err != nil {
		return err
	}
	return p.store.LearnItem(ctx, LearnedItem{ProductID: x.ProductID, ResolvedName: b.Name, SuggestedCategory: b.Category,
		Specs: b.Specs, UserExplanation: b.Explanation, Source: b.Source, RawQuery: b.Query})
}

// detailRows: the ⓘ row "AI identified · <name> — <specs>".
func (p *Plugin) detailRows(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
	it, err := p.store.GetItem(ctx, id)
	if err != nil || it == nil || it.ResolvedName == "" {
		return nil, err
	}
	v := it.ResolvedName
	if s := strings.TrimSpace(it.Specs); s != "" {
		v += " — " + s
	}
	return []plugin.DetailRow{{Label: "AI identified", Value: v}}, nil
}
