// Package pocketcashier is the Pocket Cashier app: a phone cashier on the shop's
// wifi that sells live, and from a copy kept on the phone when the PC is down.
// Core never imports it; it attaches through generic hooks — an extra HTTPS
// listener with the shop's own certificate, one person one device, the till
// copy — and its own routes at /api/v1/x/pocketcashier and pages at
// /x/pocketcashier (the phone) and /admin/x/pocketcashier (Phones).
package pocketcashier

import (
	"context"
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
	core    plugin.Core
	store   *Store
	feed    *feedHub
	tickets feedTickets
}

func (p *Plugin) Name() string                { return "Pocket Cashier app" }
func (p *Plugin) Key() string                 { return "pocketcashier" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "pocketcashier" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	p.feed = &feedHub{version: func(ctx context.Context) (string, error) { return p.core.TillCopyVersion(ctx) }}
	reg.AddDataGroup(plugin.DataGroup{Key: "pocketcashier", Name: "Pocket Cashier phones",
		Tables: []string{"pocket_ca", "pocket_phones", "pocket_sends", "pocket_actions"}})
	reg.OneDevicePerPerson()
	reg.AddTakeoverGuard(p.store.takeoverGuard)
	reg.AddAttention(p.attention)
	reg.AddListener(plugin.Listener{Key: "pocketcashier", Addr: fmt.Sprintf(":%d", Port),
		TLS: &tls.Config{GetCertificate: certSource(p.store), MinVersion: tls.VersionTLS12}})

	h := handlers{p}
	pub := reg.Public() // /api/v1/x — no sign-in; phone routes carry their own key
	pub.POST("/pocketcashier/pair", h.pair)
	pub.GET("/pocketcashier/pair/status", h.pairStatus)
	for _, f := range []string{"ca.crt", "ca.pem", "ca.mobileconfig"} {
		pub.GET("/pocketcashier/"+f, h.caFile(f))
	}
	pub.GET("/pocketcashier/ping", h.ping)
	pub.GET("/pocketcashier/sw.js", h.sw)
	pub.GET("/pocketcashier/manifest.webmanifest", h.manifest)
	pub.GET("/pocketcashier/whoami", h.whoami, h.phoneAuth)
	pub.GET("/pocketcashier/moved", h.moved, h.phoneAuth)
	pub.GET("/pocketcashier/copy", h.copyFor, h.phoneAuth)
	pub.GET("/pocketcashier/recent", h.recent, h.phoneAuth)
	pub.POST("/pocketcashier/feed-ticket", h.feedTicket, h.phoneAuth)
	pub.GET("/pocketcashier/feed", h.feed, h.feedAuth)
	pub.POST("/pocketcashier/actions", h.actions, h.phoneAuth)

	a := reg.Admin() // admin + manager; Phones is admin only
	a.GET("/pocketcashier/phones", h.phones, adminOnly)
	a.POST("/pocketcashier/phones/approve", h.approve, adminOnly)
	a.POST("/pocketcashier/phones/:id/approve", h.approve, adminOnly)
	a.POST("/pocketcashier/phones/:id/block", h.block, adminOnly)
	a.POST("/pocketcashier/phones/:id/release", h.release, adminOnly)
	a.PUT("/pocketcashier/phones/:id", h.rename, adminOnly)
	a.DELETE("/pocketcashier/phones/:id", h.remove, adminOnly)
	a.GET("/pocketcashier/addresses", h.addresses, adminOnly)
	a.GET("/pocketcashier/sends", h.sends, adminOnly)
	a.GET("/pocketcashier/sends/:id", h.sendDetail, adminOnly)
}
