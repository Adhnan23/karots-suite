// Package clearance moves slow and expiring stock before it becomes a loss, and
// retires stock that is truly dead. Core never imports it; it attaches through
// generic plugin hooks (badge, sale suggestion, info row, data group) and its
// own routes at /api/v1/x/clearance and page at /admin/x/clearance.
package clearance

import (
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
}
