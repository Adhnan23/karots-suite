// Package repairs keeps repair jobs from drop-off to pickup on top of core. A job
// is a worksheet (device, fault, parts, labour, promised date, warranty); all of
// its money goes through core: a deposit is the customer's advance (a core
// customer payment), pickup is the normal till and Pay, the outside repairer is
// paid as a core expense and a faulty part comes back through core Returns.
// Repairs writes only repair_*. Core never imports it.
package repairs

import (
	"context"
	"io/fs"
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/repairs/migrations"

	"github.com/jmoiron/sqlx"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string { return "Repairs" }
func (p *Plugin) Key() string  { return "repairs" }

// Migrations run under goose_db_version_repairs.
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "repairs" }

// dataTables is every repair_* table, parents first.
var dataTables = []string{"repair_config", "repair_jobs", "repair_parts", "repair_charges", "repair_payments",
	"repair_deposits", "repair_payouts"}

// legacyDeposit is the old deposit tender: cash a job took before deposits were advances,
// still in the books as held for it. Only Put on the bill pre-fills it.
const legacyDeposit = "deposit"

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "repairs", Name: "Repairs", Tables: dataTables})
	reg.AddProductMerger(plugin.ProductMerger{Key: "repairs", Merge: mergeProduct})
	reg.AddSaleLineHook(plugin.SaleLineHook{Key: "repairs", Record: recordPickup})
	reg.AddTenderMethod(plugin.TenderMethod{Value: legacyDeposit, Label: "Deposit held", Hidden: true, SettleTx: settleLegacyDeposit})
	p.routes(reg)
}

// settleLegacyDeposit spends a job's held old deposit on its pickup sale,
// never more than is held (the job row is locked for the check).
func settleLegacyDeposit(ctx context.Context, tx *sqlx.Tx, ev plugin.TenderSettle) error {
	jobID, _ := strconv.ParseInt(ev.Meta["job"], 10, 64)
	if jobID <= 0 {
		return apperr.Validation("a held deposit needs its repair job")
	}
	held, err := legacyHeldTx(ctx, tx, jobID)
	if err != nil {
		return err
	}
	if ev.Amount.GreaterThan(held) {
		return apperr.Validation("that's more than the deposit held for this repair")
	}
	return addPayment(ctx, tx, jobID, ev.Amount, "applied", ev.ActorID)
}

// mergeProduct moves the job lines (and the labour product) of a merged-away product.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE repair_parts SET product_id = $2 WHERE product_id = $1`,
		`UPDATE repair_config SET labour_product_id = $2 WHERE labour_product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
