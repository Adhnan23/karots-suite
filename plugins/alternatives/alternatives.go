// Package alternatives groups interchangeable products (group → optional
// quality tiers → products) so the till finds a whole group, offers an in-stock
// swap for a short line and lists alternatives in the product info, and the
// Reorder page judges stock per tier. Core never imports it; it attaches through
// generic hooks, its routes at /api/v1/x/alternatives and its React half.
package alternatives

import (
	"context"
	"encoding/json"
	"io/fs"

	"karots-suite/internal/apperr"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"
	"karots-suite/plugins/alternatives/migrations"
)

func init() { plugin.Register(&Plugin{}) }

type Plugin struct {
	core  plugin.Core
	store *Store
}

func (p *Plugin) Name() string                { return "Alternatives" }
func (p *Plugin) Key() string                 { return "alternatives" }
func (p *Plugin) Migrations() (fs.FS, string) { return migrations.FS, "alternatives" }

func (p *Plugin) Setup(reg *plugin.Registry) {
	p.core = reg.Core
	p.store = NewStore(reg.Core.DB)
	reg.AddDataGroup(plugin.DataGroup{Key: "alternatives", Name: "Alternatives / interchangeable",
		Tables: []string{"alt_groups", "alt_members", "alt_tiers"}})
	h := handlers{p}
	t := reg.Cashier() // /api/v1/x — anyone signed in: the till's swap chip
	t.GET("/alternatives/till/:pid", h.till)
	a := reg.Admin() // admin + manager
	a.GET("/alternatives", h.list)
	a.POST("/alternatives", h.createGroup)
	a.GET("/alternatives/membership", h.membership)
	a.GET("/alternatives/product/:pid", h.product)
	a.GET("/alternatives/:id", h.group)
	a.PUT("/alternatives/:id", h.renameGroup)
	a.POST("/alternatives/:id/active", h.groupActive)
	a.DELETE("/alternatives/:id", h.deleteGroup)
	a.POST("/alternatives/:id/tiers", h.createTier)
	a.PUT("/alternatives/tiers/:tid", h.updateTier)
	a.POST("/alternatives/tiers/:tid/active", h.tierActive)
	a.POST("/alternatives/tiers/:tid/move", h.moveTier)
	a.DELETE("/alternatives/tiers/:tid", h.deleteTier)
	a.POST("/alternatives/tiers/:tid/members", h.addMember)
	a.DELETE("/alternatives/members/:pid", h.removeMember)

	reg.AddProductSearchContributor(plugin.ProductSearchContributor{Match: p.store.MatchProductIDs})
	reg.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: p.store.BadgesFor})
	reg.AddProductDetailContributor(plugin.ProductDetailContributor{Rows: p.detailRows})
	reg.AddProductReorderAnnotator(plugin.ProductReorderAnnotator{Batch: p.reorderNotes, LowMembers: p.store.LowTierMemberIDs})
	reg.AddProductExtras(plugin.ProductExtras{Key: "alternatives", Validate: p.validateExtra, Save: p.saveExtra})
}

// detailRows lists a product's in-stock alternatives in the till's ⓘ popup.
func (p *Plugin) detailRows(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
	alts, err := p.store.Alternatives(ctx, id, 5)
	if err != nil || len(alts) == 0 {
		return nil, err
	}
	sym := ""
	if p.core.Settings != nil {
		if st, err := p.core.Settings.Get(ctx); err == nil {
			sym = st.CurrencySymbol
		}
	}
	rows := make([]plugin.DetailRow, 0, len(alts))
	for _, a := range alts {
		label := "Alternative"
		if a.Tier != "" {
			label = "Alternative · " + a.Tier
		}
		rows = append(rows, plugin.DetailRow{Label: label,
			Value: a.Name + " — " + money.Qty(a.Stock) + " left · " + money.Format(sym, a.SellingPrice)})
	}
	return rows, nil
}

// reorderNotes tells core's Reorder page which low products a tier still covers.
func (p *Plugin) reorderNotes(ctx context.Context, ids []int64) (map[int64]plugin.ReorderNote, error) {
	cov, err := p.store.CoverageFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]plugin.ReorderNote, len(cov))
	for id, c := range cov {
		where := c.Group
		if c.Tier != "" {
			where += " · " + c.Tier
		}
		total := money.Qty(c.Total)
		note := "⇄ " + where + " has " + total
		switch {
		case c.Low:
			note = "⇄ " + where + " is low: " + total + " (reorder at " + itoa(c.ReorderLevel) + ")"
		case c.Covered:
			note = "⇄ Covered — " + where + " has " + total
		}
		out[id] = plugin.ReorderNote{Covered: c.Covered, Note: note}
	}
	return out, nil
}

// extra is the product form's Alternatives section: the tier to sit in, 0 = none.
type extra struct {
	TierID int64 `json:"tierId"`
}

func decodeExtra(raw json.RawMessage) (extra, error) {
	var x extra
	if err := json.Unmarshal(raw, &x); err != nil {
		return x, apperr.BadRequest("invalid alternatives")
	}
	return x, nil
}

func (p *Plugin) validateExtra(ctx context.Context, _ plugin.ExtraCtx, raw json.RawMessage) error {
	x, err := decodeExtra(raw)
	if err != nil || x.TierID == 0 {
		return err
	}
	_, err = p.store.tier(ctx, x.TierID)
	return err
}

// saveExtra moves the product into the chosen tier, or out of any group.
func (p *Plugin) saveExtra(ctx context.Context, c plugin.ExtraCtx, raw json.RawMessage) error {
	x, err := decodeExtra(raw)
	if err != nil {
		return err
	}
	_, cur, err := p.store.TierOf(ctx, c.ProductID)
	if err != nil || cur == x.TierID {
		return err
	}
	if x.TierID == 0 {
		return p.store.RemoveMember(ctx, c.ProductID)
	}
	_, err = p.store.AddMember(ctx, x.TierID, c.ProductID, true)
	return err
}
