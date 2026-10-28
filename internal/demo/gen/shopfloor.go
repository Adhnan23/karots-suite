package gen

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
)

// conv is a bulk product the shop opens: from (the bag) → to (sold loose).
type conv struct {
	id       int64
	from, to int // indexes into shop.items
	ratio    float64
	loss     float64
	note     string
}

// partUse is a stock item a service sale uses (perUnit = 1/yield for yields).
type partUse struct {
	id      int64
	perUnit float64
}

// floorSetup builds the shop floor through the owner's screens: services and
// their recipes, the bags the shop opens, packs, a second barcode on some
// packaging, and the till's menu.
func (g *gen) floorSetup() error {
	fl := floorFor(g.s.p.Catalogue)
	byName := map[string]item{}
	for _, it := range g.s.items {
		byName[it.name] = it
	}
	cats, err := g.categories()
	if err != nil {
		return err
	}
	for _, sv := range fl.services {
		if _, ok := cats[sv.category]; !ok {
			var c struct {
				ID int64 `json:"id"`
			}
			if err := g.admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": sv.category}, &c); err != nil {
				return fmt.Errorf("category %s: %w", sv.category, err)
			}
			cats[sv.category] = c.ID
		}
		var p struct {
			ID int64 `json:"id"`
		}
		if err := g.admin.Do("POST", "/api/v1/admin/services", map[string]any{"name": sv.name, "categoryId": cats[sv.category],
			"unitId": g.unitID("pcs"), "sellingPrice": sv.price}, &p); err != nil {
			return fmt.Errorf("service %s: %w", sv.name, err)
		}
		var comps []map[string]any
		for _, pt := range sv.parts {
			mode := "per_unit"
			if pt.yield {
				mode = "yield"
			}
			comps = append(comps, map[string]any{"productId": byName[pt.product].id, "amount": pt.amount, "mode": mode})
		}
		var costs []map[string]string
		for _, c := range sv.costs {
			costs = append(costs, map[string]string{"label": c[0], "amount": c[1]})
		}
		if len(comps)+len(costs) > 0 {
			if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/recipe", p.ID),
				map[string]any{"components": comps, "costs": costs}, nil); err != nil {
				return fmt.Errorf("recipe %s: %w", sv.name, err)
			}
		}
	}
	for _, b := range fl.bags {
		ratio := strconv.FormatFloat(b.ratio, 'f', -1, 64)
		if err := g.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": byName[b.from].id,
			"toProductId": byName[b.to].id, "ratio": ratio, "note": b.note}, nil); err != nil {
			return fmt.Errorf("conversion %s: %w", b.from, err)
		}
	}
	for i, k := range fl.packs {
		bc := ean13(fmt.Sprintf("479%09d", 800000+i*11)) // the outer box has its own code
		if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/packs", byName[k.product].id), map[string]string{
			"name": k.name, "size": strconv.Itoa(k.size), "sellingPrice": k.price, "barcode": bc}, nil); err != nil {
			return fmt.Errorf("pack %s: %w", k.product, err)
		}
	}
	// a new packaging run: the supplier's next lot came with a different barcode
	for i, it := range g.s.items {
		if i%9 != 4 || it.unit == "kg" || it.unit == "m" {
			continue
		}
		bc := ean13(fmt.Sprintf("479%09d", 600000+i*13))
		if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/barcodes", it.id),
			map[string]string{"barcode": bc, "source": "receive"}, nil); err != nil {
			return fmt.Errorf("barcode %s: %w", it.name, err)
		}
	}
	if err := g.s.loadFloor(g.ctx, g.d); err != nil {
		return err
	}
	return g.tillMenu(fl, cats)
}

// tillMenu lays out the till's group cards: the busiest kinds of stock first,
// each with its best sellers; the mixed store has one card per department.
func (g *gen) tillMenu(fl floor, cats map[string]int64) error {
	type grp struct {
		ID int64 `json:"id"`
	}
	add := func(name, emoji string, parent int64) (int64, error) {
		var out grp
		err := g.admin.Do("POST", "/api/v1/admin/groups", map[string]any{"name": name, "emoji": emoji, "parentId": parent}, &out)
		return out.ID, err
	}
	fill := func(group int64, category string) error {
		var ids []int64
		if err := g.d.DB.SelectContext(g.ctx, &ids, `SELECT id FROM products WHERE category_id = $1 AND is_active ORDER BY id`, cats[category]); err != nil {
			return err
		}
		ranked := slices.Clone(g.s.items)
		slices.SortStableFunc(ranked, func(a, b item) int { return int(b.perWeek*100 - a.perWeek*100) })
		n := 0
		for _, it := range ranked {
			if n == 12 || !slices.Contains(ids, it.id) {
				continue
			}
			if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/groups/%d/items", group), map[string]any{"productId": it.id}, nil); err != nil {
				return err
			}
			n++
		}
		return nil
	}
	if g.s.p.Catalogue != "mixed" {
		for _, c := range fl.order {
			id, err := add(c, fl.menu[c], 0)
			if err != nil {
				return fmt.Errorf("menu %s: %w", c, err)
			}
			if err := fill(id, c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, dep := range departments {
		top, err := add(dep.name, dep.emoji, 0)
		if err != nil {
			return err
		}
		for _, c := range floors[dep.catalogue].order {
			if c == "Services" {
				continue
			}
			id, err := add(c, floors[dep.catalogue].menu[c], top)
			if err != nil {
				return fmt.Errorf("menu %s: %w", c, err)
			}
			if err := fill(id, c); err != nil {
				return err
			}
		}
	}
	id, err := add("Services", "🛎️", 0)
	if err != nil {
		return err
	}
	return fill(id, "Services")
}

// loadFloor finds the shop floor again — services, bags, packs, warranties —
// whether it was just built or restored with a past.
func (s *shop) loadFloor(ctx context.Context, d Deps) error {
	fl := floorFor(s.p.Catalogue)
	idx := map[string]int{}
	for i, it := range s.items {
		idx[it.name] = i
	}
	for name, u := range map[string]float64{"A4 Paper (sheet)": 450, "Copier Toner Cartridge": 0.2} {
		if i, ok := idx[name]; ok {
			s.items[i].use = u
		}
	}
	for _, w := range fl.warranty {
		if i, ok := idx[w.product]; ok {
			s.items[i].warranty, s.items[i].imei, s.items[i].faults = w.months, w.imei, w.faults
		}
	}
	for _, sv := range fl.services {
		var id int64
		if err := d.DB.GetContext(ctx, &id, `SELECT id FROM products WHERE name = $1 AND is_service`, sv.name); err != nil {
			return fmt.Errorf("service %s: %w", sv.name, err)
		}
		it := item{id: id, name: sv.name, price: decimal.RequireFromString(sv.price), perWeek: sv.perWeek, service: true, unit: "pcs", cat: sv.category}
		for _, pt := range sv.parts {
			per, _ := strconv.ParseFloat(pt.amount, 64)
			if pt.yield {
				per = 1 / per
			}
			it.parts = append(it.parts, partUse{id: s.items[idx[pt.product]].id, perUnit: per})
		}
		idx[sv.name] = len(s.items)
		s.items = append(s.items, it)
	}
	s.convs = nil
	for _, b := range fl.bags {
		c := conv{from: idx[b.from], to: idx[b.to], ratio: b.ratio, loss: b.loss, note: b.note}
		if err := d.DB.GetContext(ctx, &c.id, `SELECT id FROM product_conversions WHERE from_product_id = $1 AND to_product_id = $2`,
			s.items[c.from].id, s.items[c.to].id); err != nil {
			return fmt.Errorf("conversion %s: %w", b.from, err)
		}
		s.convs = append(s.convs, c)
	}
	var ids []int64
	for _, it := range s.items {
		ids = append(ids, it.id)
	}
	packs, err := products.PacksFor(ctx, d.DB, ids)
	if err != nil {
		return err
	}
	for i := range s.items {
		for _, k := range packs[s.items[i].id] {
			s.items[i].packs = append(s.items[i].packs, sales.PackPrice{Name: k.Name, Size: k.Size, Selling: k.SellingPrice, Wholesale: k.WholesalePrice})
		}
	}
	return nil
}

// bagFor is the conversion that fills item i, if it's sold loose from a bag.
func (s *shop) bagFor(i int) (conv, bool) {
	for _, c := range s.convs {
		if c.to == i {
			return c, true
		}
	}
	return conv{}, false
}

func (g *gen) categories() (map[string]int64, error) {
	var rows []struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := g.d.DB.SelectContext(g.ctx, &rows, `SELECT id, name FROM categories`); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Name] = r.ID
	}
	return out, nil
}

func (g *gen) unitID(abbr string) int64 {
	var id int64
	_ = g.d.DB.GetContext(g.ctx, &id, `SELECT id FROM units WHERE abbreviation = $1`, abbr)
	return id
}
