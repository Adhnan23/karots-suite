package gentest

import (
	"slices"
	"testing"
)

// floorTables: what a working shop floor leaves in core tables, and which
// catalogues have it (none listed = every shop).
var floorTables = map[string][]string{
	"product_groups":       nil, // the till's menu cards
	"product_group_items":  nil,
	"product_recipes":      {"grocery", "pharmacy", "phone", "mixed"},
	"product_recipe_costs": nil,
	"product_packs":        nil,
	"product_barcodes":     nil, // a second barcode on new packaging
	"held_sales":           nil,
	"product_conversions":  {"grocery", "hardware", "mixed"},
	"conversion_runs":      {"grocery", "hardware", "mixed"},
	"warranty_units":       {"phone", "hardware", "mixed"}, // a pharmacy sells a BP monitor now and then
}

func TestShopFloorFilled(t *testing.T) {
	need(t)
	for tb, cats := range floorTables {
		if cats != nil && !slices.Contains(cats, shop.Catalogue) {
			continue
		}
		if count(t, `SELECT count(*) FROM `+tb) == 0 {
			t.Errorf("%s: nothing in %s", shop.Key, tb)
		}
	}
	// services really sell, and packs ring up as packs
	if count(t, `SELECT count(*) FROM sale_items si JOIN products p ON p.id = si.product_id WHERE p.is_service AND p.name NOT LIKE '%Recharge%'`) == 0 {
		t.Error("no core service ever sold")
	}
	if count(t, `SELECT count(*) FROM sale_items si JOIN product_packs k ON k.product_id = si.product_id AND si.quantity >= k.size`) == 0 {
		t.Error("nobody ever bought a whole pack")
	}
}
