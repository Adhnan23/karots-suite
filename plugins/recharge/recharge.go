// Package recharge turns the till into a carrier mobile-money agent (Dialog eZ
// Cash, Mobitel mCash, …). Every reload, package, bill payment, wallet deposit and
// cash out is a cart line paid on the normal Pay (a cash out is a payout line);
// each device keeps one running float, and the carrier's commission is booked
// with every move, so counts only catch what's left over. Bills paid from a bank
// and cash outs into one move through core cashflow. Recharge writes only
// recharge_* and its own products. Core never imports it.
package recharge

import (
	"context"
	"io/fs"
	"time"

	"karots-suite/internal/plugin"
	"karots-suite/plugins/recharge/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string { return "Reload & Bills" }
func (p *Plugin) Key() string  { return "recharge" }

// Migrations run under goose_db_version_recharge (karots-pos's, so its backups restore).
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "recharge" }

// dataTables is every recharge_* table, parents first (karots-pos's retired bank
// card tables stay listed so an old backup carrying them restores).
var dataTables = []string{"recharge_carriers", "recharge_devices", "recharge_topups", "recharge_carrier_sessions",
	"recharge_device_sessions", "recharge_transactions", "recharge_bill_tx", "recharge_bank_cards", "recharge_card_tx",
	"recharge_rates", "recharge_packages", "recharge_counts", "recharge_config"}

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "recharge", Name: "Reload & Bills", Tables: dataTables})
	reg.AddProductMerger(plugin.ProductMerger{Key: "recharge", Merge: mergeProduct})
	reg.AddPLIncome(plugin.PLIncome{Label: "Reload & Bills earnings", Amount: func(ctx context.Context, from, to time.Time) (decimal.Decimal, error) {
		return Earnings(ctx, p.store.db, from, to)
	}})
	reg.AddPLIncome(plugin.PLIncome{Label: "Float bought earlier — not a cost", Amount: func(ctx context.Context, from, to time.Time) (decimal.Decimal, error) {
		return OldFloatBought(ctx, p.store.db, from, to)
	}})
	reg.AddExpenseOwner(p.store.expenseOwner)
	reg.AddMoneyLocationProvider(plugin.MoneyLocationProvider{Group: "Reload float", List: p.store.moneyLocations})
	reg.AddReadinessProvider(p.readiness)
	reg.AddActivityContributor(plugin.ActivityContributor{Source: "recharge", List: p.store.activityRows})
	reg.AddSaleLineHook(plugin.SaleLineHook{Key: "recharge", Record: p.recordSale})
	reg.AddReturnLineHook(plugin.ReturnLineHook{Key: "recharge", Record: p.recordReturn})
	reg.AddTenderMethod(plugin.TenderMethod{Value: walletMethod, Label: "Wallet (eZ Cash / mCash)", SettleTx: p.settleWallet})
}

// walletMethod is karots-pos's payment_method value (its migration 00003 added it).
const walletMethod = "wallet"

// mergeProduct follows a carrier's or the plugin's own product into the kept one.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE recharge_carriers SET product_id = $2 WHERE product_id = $1`,
		`UPDATE recharge_config SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
