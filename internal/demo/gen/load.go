package gen

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/stock"
)

// loadShop finds a built shop's people, products, suppliers, customers and
// accounts again (a restored past), matching them to the catalogue by name.
func loadShop(ctx context.Context, d Deps, p shops.Profile) (*shop, error) {
	s := &shop{p: p}
	lines, sups := linesFor(p.Catalogue)
	for _, l := range sups {
		var id int64
		if err := d.DB.GetContext(ctx, &id, `SELECT id FROM suppliers WHERE name = $1`, strings.Split(l, ";")[0]); err != nil {
			return nil, fmt.Errorf("supplier %s: %w", l, err)
		}
		s.suppliers = append(s.suppliers, id)
	}
	cats := map[string]int{}
	for _, l := range strings.Split(strings.TrimSpace(lines), "\n") {
		f := strings.Split(l, ";")
		if _, ok := cats[f[0]]; !ok {
			cats[f[0]] = len(cats)
		}
		var id int64
		if err := d.DB.GetContext(ctx, &id, `SELECT id FROM products WHERE name = $1 AND is_active`, f[1]); err != nil {
			return nil, fmt.Errorf("product %s: %w", f[1], err)
		}
		pw, _ := strconv.ParseFloat(f[6], 64)
		s.items = append(s.items, item{id: id, name: f[1], cost: decimal.RequireFromString(f[4]),
			price: decimal.RequireFromString(f[5]), perWeek: pw, supplier: cats[f[0]] % len(s.suppliers), unit: f[3]})
	}
	for _, l := range strings.Split(customerLines, "\n") {
		var id int64
		if err := d.DB.GetContext(ctx, &id, `SELECT id FROM customers WHERE phone = $1`, strings.Split(l, ";")[1]); err != nil {
			return nil, fmt.Errorf("customer %s: %w", l, err)
		}
		s.customers = append(s.customers, id)
	}
	if err := d.DB.GetContext(ctx, &s.safe, `SELECT id FROM lockers WHERE kind = 'safe' ORDER BY id LIMIT 1`); err != nil {
		return nil, fmt.Errorf("safe: %w", err)
	}
	if err := d.DB.GetContext(ctx, &s.card, `SELECT id FROM lockers WHERE kind = 'card' ORDER BY id LIMIT 1`); err != nil {
		return nil, fmt.Errorf("card account: %w", err)
	}
	if err := d.DB.GetContext(ctx, &s.bank, `SELECT id FROM lockers WHERE kind = 'bank' ORDER BY id LIMIT 1`); err != nil {
		return nil, fmt.Errorf("bank account: %w", err)
	}
	return s, s.loadPeople(ctx, d)
}

// loadStock reads what's on the shelf, so today's restock is right.
func (g *gen) loadStock() error {
	st := stock.NewService(g.d.DB)
	for _, it := range g.s.items {
		q, err := st.Quantity(g.ctx, it.id)
		if err != nil {
			return err
		}
		g.stock[it.id], _ = q.Float64()
	}
	return nil
}
