package gen

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Combos: a few bundles at a percent off, rung as a group of lines (as the till's
// combo panel adds them) and recorded as sold after the sale.

type combo struct {
	id      int64
	name    string
	percent int64
	members []item
}

var comboDefs = []struct {
	name    string
	percent int64
	members []string
}{
	{"Phone starter pack", 15, []string{"Tempered Glass", "Silicone Back Cover", "USB-C Cable"}},
	{"Fast charge bundle", 10, []string{"20W USB-C Fast Charger", "USB-C Cable"}},
	{"Tea time", 10, []string{"Ceylon Tea", "Marie Biscuits", "White Sugar"}},
	{"Rice & curry pack", 8, []string{"Samba Rice", "Mysore Dhal", "Curry Powder"}},
}

func combosSetup(g *gen) error {
	for _, d := range comboDefs {
		var members []item
		for _, m := range d.members {
			if it, ok := g.itemNamed(m); ok {
				members = append(members, it)
			}
		}
		if len(members) != len(d.members) {
			continue // not this shop's stock
		}
		var out struct {
			ID int64 `json:"id"`
		}
		body := map[string]any{"name": d.name, "discountType": "percent", "discountValue": fmt.Sprint(d.percent), "startsOn": "", "endsOn": "", "items": toItems(members)}
		if err := g.admin.Do("POST", "/api/v1/x/combos", body, &out); err != nil {
			return err
		}
		g.combos = append(g.combos, combo{out.ID, d.name, d.percent, members})
	}
	return nil
}

func toItems(ms []item) []map[string]any {
	var out []map[string]any
	for _, m := range ms {
		out = append(out, map[string]any{"productId": m.id, "qty": "1"})
	}
	return out
}

func combosDaily(g *gen, day time.Time, until float64) error {
	if len(g.combos) == 0 {
		if err := g.loadCombos(); err != nil || len(g.combos) == 0 {
			return err
		}
	}
	for i := 0; i < g.r.IntN(4); i++ {
		h := 10 + g.r.Float64()*9
		if h >= until {
			continue
		}
		c := g.combos[g.r.IntN(len(g.combos))]
		ok := true
		for _, m := range c.members {
			ok = ok && g.stock[m.id] >= 1
		}
		if !ok {
			continue
		}
		g.tm.at(at(day, h))
		if err := g.comboSale(c); err != nil {
			return err
		}
	}
	return nil
}

func (g *gen) comboSale(c combo) error {
	var items []map[string]any
	gross, net := decimal.Zero, decimal.Zero
	for _, m := range c.members {
		g.stock[m.id]--
		price := g.priceOf(m.id)
		gross = gross.Add(price)
		net = net.Add(price.Mul(decimal.NewFromInt(100 - c.percent)).Div(decimal.NewFromInt(100)))
		items = append(items, map[string]any{"productId": m.id, "quantity": "1", "discount": fmt.Sprint(c.percent), "discountType": "percent",
			"group": map[string]any{"no": 1, "label": c.name + " (combo)", "sets": "1"}, "plugin": map[string]any{"combos": map[string]any{"comboId": c.id}}})
	}
	saleID, err := g.tillSale(g.cashier(), 0, items, net.Round(2))
	if err != nil {
		return err
	}
	return g.lastTill.Do("POST", "/api/v1/x/combos/sold", map[string]any{"saleId": saleID, "combos": []map[string]any{
		{"comboId": c.id, "sets": "1", "net": net.StringFixed(2), "discount": gross.Sub(net).StringFixed(2)}}}, nil)
}

// loadCombos finds a restored shop's combos again.
func (g *gen) loadCombos() error {
	var rows []struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Value string `db:"discount_value"`
	}
	if err := g.d.DB.SelectContext(g.ctx, &rows, `SELECT id, name, discount_value::int::text AS discount_value FROM combos ORDER BY id`); err != nil {
		return err
	}
	for _, r := range rows {
		for _, d := range comboDefs {
			if d.name != r.Name {
				continue
			}
			var ms []item
			for _, m := range d.members {
				if it, ok := g.itemNamed(m); ok {
					ms = append(ms, it)
				}
			}
			g.combos = append(g.combos, combo{r.ID, r.Name, d.percent, ms})
		}
	}
	return nil
}
