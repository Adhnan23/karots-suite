// Package productplus is the custom product fields plugin: an admin defines extra
// fields (text, long text, number, yes/no, dropdown, date) for all products or
// chosen categories; they show on the product form and intake, the till ⓘ,
// labels, search, the product list and sheets, and Fill in sets them across many
// products. Core never imports it; it attaches through generic hooks, its routes
// at /api/v1/x/productplus and its page at /admin/x/productplus.
package productplus

import (
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/productplus/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Product Plus" }
func (p *Plugin) Key() string                 { return "productplus" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "productplus" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "productplus", Name: "Product Plus (custom fields)",
		Tables: []string{"pp_fields", "pp_values", "pp_field_categories"}})
}
