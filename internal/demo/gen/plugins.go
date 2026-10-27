package gen

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// A plugin's life in the demo: set up once with the shop, then a little each
// trading day — always through the plugin's own endpoints (and the till's, for
// what it sells), the same calls its screens make. Plugins carry no demo code.
type pluginGen struct {
	setup func(g *gen) error
	daily func(g *gen, day time.Time, until float64) error // until: last hour traded (today < 20.5)
}

var pluginGens = map[string]pluginGen{
	"repairs":  {setup: repairsSetup, daily: repairsDaily},
	"recharge": {setup: rechargeSetup, daily: rechargeDaily},
}

func (g *gen) on(key string) bool { return slices.Contains(g.s.p.Plugins, key) }

func (g *gen) pluginsSetup() error {
	for _, k := range g.s.p.Plugins {
		if pg, ok := pluginGens[k]; ok && pg.setup != nil {
			if err := pg.setup(g); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	}
	return nil
}

func (g *gen) pluginsDaily(day time.Time, until float64) error {
	for _, k := range g.s.p.Plugins {
		if pg, ok := pluginGens[k]; ok && pg.daily != nil {
			if err := pg.daily(g, day, until); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	}
	return nil
}

// tillOf is a signed-in till client for a cashier (one sign-in each per run).
func (g *gen) tillOf(c person) (*Client, error) {
	if cl, ok := g.tills[c.id]; ok {
		return cl, nil
	}
	cl, err := NewClient(g.d.API, c.login.Phone, c.login.PIN, "demo-till-"+strconv.FormatInt(c.id, 10))
	if err != nil {
		return nil, err
	}
	g.tills[c.id] = cl
	return cl, nil
}

// tillSale rings items up at the till as cashier c and pays it in cash (or by
// card), exactly as the Pay screen sends it.
func (g *gen) tillSale(c person, customerID int64, items []map[string]any, total decimal.Decimal) (int64, error) {
	cl, err := g.tillOf(c)
	if err != nil {
		return 0, err
	}
	pay := map[string]any{"method": "cash", "amount": tendered(g.r, total).String()}
	if g.r.IntN(4) == 0 {
		pay = map[string]any{"method": "card", "amount": total.String(), "accountId": g.s.card}
	}
	in := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "items": items, "payments": []any{pay}}
	if customerID > 0 {
		in["customerId"] = customerID
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := cl.Do("POST", "/api/v1/till/sales", in, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// cashier is whoever is on a till today.
func (g *gen) cashier() person { return g.open[g.r.IntN(len(g.open))] }

func (g *gen) customer() int64 { return g.s.customers[g.r.IntN(len(g.s.customers))] }

// itemNamed finds a catalogue product by name prefix.
func (g *gen) itemNamed(prefix string) (item, bool) {
	for _, it := range g.s.items {
		if len(it.name) >= len(prefix) && it.name[:len(prefix)] == prefix {
			return it, true
		}
	}
	return item{}, false
}

// priceOf is a product's shelf price as the till would charge it.
func (g *gen) priceOf(id int64) decimal.Decimal {
	var p decimal.Decimal
	_ = g.d.DB.GetContext(g.ctx, &p, `SELECT selling_price FROM products WHERE id = $1`, id)
	return p
}
