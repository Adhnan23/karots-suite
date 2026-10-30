// Package pocketcashier is the Pocket Cashier app: a phone cashier on the shop's
// wifi that sells live, and from a copy kept on the phone when the PC is down.
// Core never imports it; it attaches through generic hooks — an extra HTTPS
// listener with the shop's own certificate, one person one device, the till
// copy — and its own routes at /api/v1/x/pocketcashier and pages at
// /x/pocketcashier (the phone) and /admin/x/pocketcashier (Phones).
package pocketcashier

import (
	"crypto/tls"
	"fmt"
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/pocketcashier/migrations"
)

// Port is the phones' HTTPS port.
const Port = 8443

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Pocket Cashier app" }
func (p *Plugin) Key() string                 { return "pocketcashier" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "pocketcashier" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "pocketcashier", Name: "Pocket Cashier phones",
		Tables: []string{"pocket_ca", "pocket_phones", "pocket_sends", "pocket_actions"}})
	reg.OneDevicePerPerson()
	reg.AddListener(plugin.Listener{Key: "pocketcashier", Addr: fmt.Sprintf(":%d", Port),
		TLS: &tls.Config{GetCertificate: certSource(p.store), MinVersion: tls.VersionTLS12}})
}
