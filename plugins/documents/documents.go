package documents

import (
	"io/fs"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/documents/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string { return "Print & Copy" }
func (p *Plugin) Key() string  { return "documents" }

// Migrations run under goose_db_version_documents (karots-pos's, so its backups restore).
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "documents" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "documents", Name: "Print & Copy",
		Tables: []string{"doc_service", "doc_price", "doc_consumable", "doc_job", "doc_payout"}})
	h := handlers{p}
	t := reg.Cashier() // /api/v1/x — anyone signed in: the till
	t.GET("/documents/till", h.till)
	t.GET("/documents/quote", h.quote)
	t.POST("/documents/own-use", h.ownUse)
	t.POST("/documents/jobs", h.jobs)
	a := reg.Admin() // admin + manager
	a.GET("/documents/services", h.services)
	a.POST("/documents/services", h.create)
	a.PUT("/documents/services/:id", h.update)
	a.POST("/documents/services/:id/active", h.setActive)
	a.GET("/documents/services/:id/grid", h.grid)
	a.PUT("/documents/services/:id/grid", h.saveGrid)
	a.GET("/documents/report", h.report)
	// 🖨 on the till tile; the till's Print & Copy menu claims the product by it.
	reg.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: p.store.Badges})
}
