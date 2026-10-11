// Package combos lets a shop define product deals — a group of products at %
// off, Rs off or a set price, between optional dates — that ring as REAL
// discounted product lines kept together in the cart, so they return and
// restock like items sold one by one. Core never imports it; it attaches
// through generic hooks, its routes at /api/v1/x/combos and its React half.
package combos

import (
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/combos/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Combos" }
func (p *Plugin) Key() string                 { return "combos" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "combos" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "combos", Name: "Combos / deals",
		Tables: []string{"combos", "combo_items", "combo_sales"}})
	// Flag member products in till search so the cashier knows a combo exists.
	reg.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: p.store.BadgesFor})
}
