// Package clearance moves slow and expiring stock before it becomes a loss, and
// retires stock that is truly dead. Core never imports it; it attaches through
// generic plugin hooks (badge, sale suggestion, info row, data group) and its
// own routes at /api/v1/x/clearance and page at /admin/x/clearance.
package clearance

import (
	"context"
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/clearance/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Clearance" }
func (p *Plugin) Key() string                 { return "clearance" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "clearance" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)

	reg.AddDataGroup(plugin.DataGroup{Key: "clearance", Name: "Clearance / markdowns",
		Tables: []string{"clearance_markdowns", "clearance_settings"}})
	reg.AddProductMerger(plugin.ProductMerger{Key: "clearance", Merge: mergeProduct})
	// The till: card badge, apply / skip prompt and ⓘ row — live markdowns only.
	reg.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: p.store.BadgesFor})
	reg.AddProductSaleSuggestionProvider(plugin.ProductSaleSuggestionProvider{Batch: p.store.SuggestionsFor})
	reg.AddProductDetailContributor(plugin.ProductDetailContributor{
		Rows: func(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
			m, err := p.store.live(ctx, []int64{id})
			if err != nil || len(m) == 0 {
				return nil, err
			}
			return []plugin.DetailRow{{Label: "Clearance", Value: m[id].label()}}, nil
		},
	})

	h := handlers{p}
	a := reg.Admin() // /api/v1/x — admin + manager
	a.GET("/clearance", h.page)
	a.PUT("/clearance/settings", h.saveSettings)
	a.POST("/clearance/:pid/approve", h.approve)
	a.POST("/clearance/:pid/snooze", h.snooze)
	a.POST("/clearance/:pid/unsnooze", h.unsnooze)
	a.POST("/clearance/disable", h.disable)
}
