// Package requests keeps what customers asked for that the shop doesn't carry:
// repeat asks add up, an admin makes or links a product or says it won't be
// stocked, and once the item is in the list says whom to call. Core never
// imports it; it attaches through generic hooks, its routes at /api/v1/x/requests
// and its page at /admin/x/requests.
package requests

import (
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/requests/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Customer requests" }
func (p *Plugin) Key() string                 { return "requests" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "requests" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "requests", Name: "Customer requests / wanted",
		Tables: []string{"requests", "request_asks"}})
}
