package api

import (
	"context"
	"strconv"

	"karots-suite/internal/db"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// A products sheet's changes, old → new (docs/rules/stock.md IMP-4): each
// product's fields as the sheet names them, read before the import and again
// after each line, through the import's own transaction.

type productSnap map[string]string

// snapFields is the order changes list in: the sheet's own columns, then the
// plugin columns a sheet asked for.
var snapFields = []string{"name", "name_local", "barcode", "extra_barcodes", "category", "unit", "cost_price",
	"selling_price", "wholesale_price", "tax_rate", "reorder_level", "supplier", "track_serial", "warranty_months", "active"}

// productSnaps reads ids' fields (nil = every product).
func productSnaps(ctx context.Context, q db.Queryer, ids []int64, xs []sheetExtra) (map[int64]productSnap, error) {
	var rows []struct {
		ID        int64           `db:"id"`
		Name      string          `db:"name"`
		Local     string          `db:"name_local"`
		Barcode   string          `db:"barcode"`
		Extra     string          `db:"extra_barcodes"`
		Category  int64           `db:"category_id"`
		Unit      string          `db:"unit"`
		Cost      decimal.Decimal `db:"cost_price"`
		Selling   decimal.Decimal `db:"selling_price"`
		Wholesale decimal.Decimal `db:"wholesale_price"`
		Tax       decimal.Decimal `db:"tax_rate"`
		Reorder   int             `db:"reorder_level"`
		Supplier  string          `db:"supplier"`
		Serial    bool            `db:"track_serial"`
		Warranty  int             `db:"warranty_months"`
		Active    bool            `db:"is_active"`
	}
	if err := q.SelectContext(ctx, &rows, `
		SELECT p.id, p.name, COALESCE(p.name_local, '') AS name_local, COALESCE(p.barcode, '') AS barcode,
		       COALESCE((SELECT string_agg(b.barcode, ', ' ORDER BY b.barcode) FROM product_barcodes b
		                 WHERE b.product_id = p.id AND b.pack_id IS NULL), '') AS extra_barcodes,
		       p.category_id, u.name AS unit, p.cost_price, p.selling_price, p.wholesale_price, p.tax_rate,
		       p.reorder_level, COALESCE(s.name, '') AS supplier, p.track_serial, p.warranty_months, p.is_active
		FROM products p JOIN units u ON u.id = p.unit_id
		LEFT JOIN suppliers s ON s.id = p.preferred_supplier_id
		WHERE $1::bigint[] IS NULL OR p.id = ANY($1::bigint[])`, pq.Array(ids)); err != nil {
		return nil, err
	}
	paths, err := categoryPaths(ctx, q)
	if err != nil {
		return nil, err
	}
	yes := map[bool]string{true: "yes", false: "no"}
	out := make(map[int64]productSnap, len(rows))
	got := make([]int64, 0, len(rows))
	for _, r := range rows {
		out[r.ID] = productSnap{"name": r.Name, "name_local": r.Local, "barcode": r.Barcode, "extra_barcodes": r.Extra,
			"category": paths[r.Category], "unit": r.Unit, "cost_price": r.Cost.String(), "selling_price": r.Selling.String(),
			"wholesale_price": r.Wholesale.String(), "tax_rate": r.Tax.String(), "reorder_level": strconv.Itoa(r.Reorder),
			"supplier": r.Supplier, "track_serial": yes[r.Serial], "warranty_months": strconv.Itoa(r.Warranty), "active": yes[r.Active]}
		got = append(got, r.ID)
	}
	for _, x := range xs { // plugin columns, read through the transaction ctx carries
		vals, err := x.hook.Export(ctx, got)
		if err != nil {
			return nil, err
		}
		for id, cells := range vals {
			for i, col := range x.cols {
				if snap := out[id]; snap != nil && i < len(cells) {
					snap[col] = cells[i]
				}
			}
		}
	}
	return out, nil
}

// diffSnaps lists the fields that differ, in snapFields order then the plugin columns'.
func diffSnaps(before, after productSnap, xs []sheetExtra) []FieldChange {
	out := []FieldChange{}
	order := append([]string{}, snapFields...)
	for _, x := range xs {
		order = append(order, x.cols...)
	}
	for _, f := range order {
		if before[f] != after[f] {
			out = append(out, FieldChange{Field: f, From: before[f], To: after[f]})
		}
	}
	return out
}

// categoryPaths is every category's "Parent > Child" path.
func categoryPaths(ctx context.Context, q db.Queryer) (map[int64]string, error) {
	var cs []struct {
		ID     int64  `db:"id"`
		Name   string `db:"name"`
		Parent *int64 `db:"parent_id"`
	}
	if err := q.SelectContext(ctx, &cs, `SELECT id, name, parent_id FROM categories`); err != nil {
		return nil, err
	}
	by := make(map[int64]int, len(cs))
	for i, c := range cs {
		by[c.ID] = i
	}
	out := make(map[int64]string, len(cs))
	var path func(id int64, depth int) string
	path = func(id int64, depth int) string {
		if p, ok := out[id]; ok {
			return p
		}
		i, ok := by[id]
		if !ok || depth > 32 {
			return ""
		}
		p := cs[i].Name
		if cs[i].Parent != nil {
			if up := path(*cs[i].Parent, depth+1); up != "" {
				p = up + " > " + p
			}
		}
		out[id] = p
		return p
	}
	for _, c := range cs {
		path(c.ID, 0)
	}
	return out, nil
}
