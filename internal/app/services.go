package app

import (
	"context"
	"time"

	"karots-suite/internal/config"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// Services are the domain services the JSON API calls, with every core seam wired
// exactly as karots-pos wired them. Built once in BuildServer. Areas add a field
// here as they are ported (M1+); the legacy per-feature RegisterAPI routes are
// NOT mounted — each area's port deletes its legacy api.go.
type Services struct {
	Auth         *auth.Service
	Settings     *settings.Service
	Sales        *sales.Service
	CashRegister *cashregister.Service
	Cashflow     *cashflow.Service
	KickDrawer   func(context.Context)
}

// NewServices builds every domain service with the same core seams karots-pos
// wired in BuildServer, but via NewService rather than RegisterAPI, so no legacy
// route gets mounted. systemPIN is the login check for the hidden System account.
func NewServices(db *sqlx.DB, cfg *config.Config, systemPIN func(pin string, now time.Time) bool) *Services {
	st := settings.NewService(db)
	kick := newDrawerKicker(st)
	// One cashflow service backs both the till-locker hook (cash events) and the
	// sale non-cash hook (card/online tenders land in a card/bank account).
	cf := cashflow.NewService(db, sales.NewService(db))
	// Plugin tenders (e.g. recharge "wallet") settle their own ledger inside the
	// sale's tx. The dispatcher bridges core→plugin at the composition root, so
	// core sales never imports any plugin; a method with no registered SettleTx is
	// simply recorded on the sale.
	pluginTenderLeg := func(ctx context.Context, tx *sqlx.Tx, ev sales.PluginTender) error {
		for _, t := range plugin.TenderMethods() {
			if t.Value == ev.Method && t.SettleTx != nil {
				return t.SettleTx(ctx, tx, plugin.TenderSettle{
					SaleID: ev.SaleID, Amount: ev.Amount, Meta: ev.Meta, ActorID: ev.ActorID,
				})
			}
		}
		return nil
	}
	salesSvc := sales.NewService(db).WithDrawerKick(kick).WithNonCashLeg(cf.SaleNonCashLeg).WithPluginTenderLeg(pluginTenderLeg)
	cr := cashregister.NewService(db, sales.NewService(db)).WithAudit(audit.NewService(db))
	// Wire the locker side of till cash events (opening float from a locker,
	// banking at close, mid-shift moves to/from a locker) through cashflow, which
	// imports cashregister — so the hook is injected here rather than imported.
	cr.WithLockerLeg(cf.TillLockerLeg)
	cr.WithDrawerKick(kick)
	// Same shape: settling a new invoice out of a supplier's advances and return
	// credits lives in supplierpay, which imports purchases — so purchases takes
	// it as a hook rather than importing back. Wiring it here means every path
	// that books a payable spends the credit, with no caller left to forget.
	purchases.WithSettleCredit(supplierpay.NewService(db).ApplySupplierCreditTx)

	a := auth.NewService(db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	a.WithSystemPINValidator(systemPIN)
	return &Services{Auth: a, Settings: st, Sales: salesSvc, CashRegister: cr, Cashflow: cf, KickDrawer: kick}
}

// newDrawerKicker builds the best-effort action that pops the physical cash
// drawer wired to the receipt printer. It reads the current settings on each call
// (so toggling the feature takes effect without a restart) and is injected into
// the services that own a till cash event — cash register and sales. A no-op when
// the shop hasn't enabled a drawer.
//
// The pulse fires on a detached goroutine so it never blocks the caller's HTTP
// response, with a timeout so a dead printer can't leak the goroutine.
func newDrawerKicker(settingsSvc *settings.Service) func(context.Context) {
	return func(ctx context.Context) {
		bg := context.WithoutCancel(ctx)
		go func() {
			c, cancel := context.WithTimeout(bg, 20*time.Second)
			defer cancel()
			cfg, err := settingsSvc.Get(c)
			if err != nil || cfg == nil {
				return
			}
			escpos.KickDrawer(c, *cfg)
		}()
	}
}
