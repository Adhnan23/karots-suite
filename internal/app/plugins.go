package app

import (
	"context"

	"karots-suite/internal/config"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// startPlugins builds the registry over the same wired services core uses (so a
// plugin's sale, expense or money move is identical to a core one), runs every
// compiled-in plugin's Setup, and feeds the product func-vars the till reads from
// the plugin hooks. Returns the route mounter for api.Deps.MountPlugins.
func startPlugins(e *echo.Echo, db *sqlx.DB, cfg *config.Config, svc *Services) func(public, cashier, admin *echo.Group) {
	mux := plugin.NewMux()
	reg := plugin.NewRegistry(plugin.Core{
		DB: db, Cfg: cfg,
		Audit:        audit.NewService(db),
		Settings:     svc.Settings,
		CashRegister: svc.CashRegister,
		Sales:        svc.Sales,
		Expenses:     expenses.NewService(db),
		Products:     products.NewService(db),
		Stock:        stock.NewService(db),
		Cashflow:     svc.Cashflow,
		Lockers:      lockers.NewService(db),
	}, mux, e)
	plugin.SetupAll(reg)
	bridgeProductHooks()
	return mux.Mount
}

// bridgeProductHooks sets each products func-var only when some plugin provides
// it (a core-only build leaves them nil). Every provider's result is merged; a
// provider that errors is skipped, so a plugin issue never breaks the till.
func bridgeProductHooks() {
	if ps := plugin.ProductBadgeProviders(); len(ps) > 0 {
		products.BadgeProvider = func(ctx context.Context, ids []int64) map[int64][]string {
			out := map[int64][]string{}
			for _, p := range ps {
				m, err := p.Batch(ctx, ids)
				if err != nil {
					continue
				}
				for id, b := range m {
					out[id] = append(out[id], b...)
				}
			}
			return out
		}
	}
	if ps := plugin.ProductSaleSuggestionProviders(); len(ps) > 0 {
		products.SaleSuggestionProvider = func(ctx context.Context, ids []int64) map[int64]products.SaleSuggestion {
			out := map[int64]products.SaleSuggestion{}
			for _, p := range ps {
				m, err := p.Batch(ctx, ids)
				if err != nil {
					continue
				}
				for id, s := range m {
					if _, taken := out[id]; !taken { // first plugin to suggest wins: one prompt per line
						out[id] = products.SaleSuggestion{DiscountType: s.DiscountType, DiscountValue: s.DiscountValue, Label: s.Label, Prompt: s.Prompt}
					}
				}
			}
			return out
		}
	}
	if ps := plugin.ProductMetaProviders(); len(ps) > 0 {
		products.MetaProvider = func(ctx context.Context, ids []int64) map[int64]string {
			out := map[int64]string{}
			for _, p := range ps {
				m, err := p.Batch(ctx, ids)
				if err != nil {
					continue
				}
				for id, s := range m {
					if out[id] != "" {
						s = out[id] + " · " + s
					}
					out[id] = s
				}
			}
			return out
		}
	}
	if ps := plugin.ProductDetailContributors(); len(ps) > 0 {
		products.DetailContributor = func(ctx context.Context, id int64) []products.DetailRow {
			return detailRows(ctx, id, func(p plugin.ProductDetailContributor) func(context.Context, int64) ([]plugin.DetailRow, error) { return p.Rows }, ps)
		}
	}
	if ps := plugin.ProductLabelFieldProviders(); len(ps) > 0 {
		products.LabelFieldProvider = func(ctx context.Context, id int64) []products.DetailRow {
			return detailRows(ctx, id, func(p plugin.ProductLabelFieldProvider) func(context.Context, int64) ([]plugin.DetailRow, error) { return p.Rows }, ps)
		}
	}
	if ps := plugin.ProductSearchContributors(); len(ps) > 0 {
		products.SearchContributor = func(ctx context.Context, q string) ([]int64, error) {
			var out []int64
			for _, p := range ps {
				if ids, err := p.Match(ctx, q); err == nil {
					out = append(out, ids...)
				}
			}
			return out, nil
		}
	}
}

func detailRows[P any](ctx context.Context, id int64, rows func(P) func(context.Context, int64) ([]plugin.DetailRow, error), ps []P) []products.DetailRow {
	var out []products.DetailRow
	for _, p := range ps {
		rs, err := rows(p)(ctx, id)
		if err != nil {
			continue
		}
		for _, r := range rs {
			out = append(out, products.DetailRow{Label: r.Label, Value: r.Value})
		}
	}
	return out
}
