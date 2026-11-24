// Package catalogue keeps the shop's copy of the Karots Suite catalogue (published online by the Catalogue app)
// on the main PC, searches it, and lets a product be made from a pick: the name, unit and the shop's own
// Product Plus fields fill in; picks and later catalogue changes wait on Items to review. Rules: CTL in
// docs/rules/plugins/catalogue.md. Core never imports it; routes at /api/v1/x/catalogue, page /admin/x/catalogue.
package catalogue

import (
	"io/fs"
	"os"
	"strings"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/catalogue/migrations"
)

func init() { plugin.Register(&Plugin{}) }

// baseURL is the published catalogue's folder, baked in by the Shop Builder
// (-X karots-suite/plugins/catalogue.baseURL=…); KAROTS_CATALOGUE_URL overrides it.
var baseURL = ""

// address is where this shop's catalogue comes from ("" = none in this build).
func address() string {
	if u := os.Getenv("KAROTS_CATALOGUE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return strings.TrimRight(baseURL, "/")
}

type Plugin struct {
	core  plugin.Core
	store *Store
	sync  *Syncer
}

func (p *Plugin) Name() string                { return "Catalogue" }
func (p *Plugin) Key() string                 { return "catalogue" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "catalogue" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	p.sync = NewSyncer(reg.Core.DB, address())
	p.sync.afterChange = p.store.afterChange
	reg.AddBackground(p.sync.Loop)
	reg.AddProductExtras(p.store.extras())
	reg.AddReviewSource(p.store.reviewSource())
	reg.AddDataGroup(plugin.DataGroup{Key: "catalogue", Name: "Catalogue (the shop's copy, links, reviews)",
		Tables: []string{"catalogue_items", "catalogue_state", "catalogue_links", "catalogue_reviews"}})

	h := handlers{p}
	t := reg.Cashier() // anyone signed in: the till's quick item, Pocket Cashier, Shelf
	t.GET("/catalogue/status", h.status)
	t.GET("/catalogue/search", h.search)
	a := reg.Admin()
	a.POST("/catalogue/check", h.check)
	a.GET("/catalogue/reviews/:product", h.waiting)
	a.POST("/catalogue/reviews/:product/use", h.use)
	a.POST("/catalogue/reviews/:product/keep", h.keep)
}
