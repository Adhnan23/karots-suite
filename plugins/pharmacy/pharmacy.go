// Package pharmacy keeps a pharmacy's records on top of core: patients (a core
// customer + a clinical profile), prescriptions, dispenses recorded from the sale
// they were sold on, refill reminders and a controlled-drug register read from
// core stock movements. Medicines are ordinary products counted in tablets with a
// Card pack (core packs); generic name / strength are the shop's own Product Plus
// fields and a medicine group is an Alternatives group — pharmacy reads both and
// writes only pharmacy_*. Core never imports it.
package pharmacy

import (
	"io/fs"

	"karots-suite/internal/plugin"
	_ "karots-suite/plugins/alternatives" // always shipped together (plugin.json requires)
	"karots-suite/plugins/pharmacy/migrations"
	_ "karots-suite/plugins/productplus"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string { return "Pharmacy" }
func (p *Plugin) Key() string  { return "pharmacy" }

// Migrations run under goose_db_version_pharmacy (karots-pos's, so its backups restore).
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "pharmacy" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "pharmacy", Name: "Pharmacy", Tables: dataTables})
	reg.AddProductMerger(plugin.ProductMerger{Key: "pharmacy", Merge: mergeProduct})
	// No CategoryMerger: pharmacy keeps no category ids.
	h := handlers{p}
	t := reg.Cashier() // /api/v1/x — anyone signed in: the till's 💊 panel and the pages
	t.GET("/pharmacy/patients", h.search)
	t.POST("/pharmacy/patients", h.register)
	t.POST("/pharmacy/patients/from-customer/:cid", h.fromCustomer)
	t.GET("/pharmacy/patients/:id", h.patient)
	t.PUT("/pharmacy/patients/:id", h.updatePatient)
	t.GET("/pharmacy/patients/:id/scripts", h.scripts)
	t.POST("/pharmacy/prescriptions", h.createScript)
	t.PUT("/pharmacy/prescriptions/:id", h.updateScript)
	t.POST("/pharmacy/prescriptions/:id/active", h.scriptActive)
}

// dataTables is every pharmacy_* table, parents first.
var dataTables = []string{
	"pharmacy_patients", "pharmacy_prescriptions", "pharmacy_prescription_items",
	"pharmacy_dispenses", "pharmacy_dispense_items",
	"pharmacy_controlled_products", "pharmacy_cd_ledger", "pharmacy_medicines",
}
