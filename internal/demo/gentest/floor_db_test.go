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
	// the back room
	"purchase_returns": nil,
	"loss_recoveries":  nil,
	"warranty_claims":  {"phone", "mixed"},
}

// pluginLife: what each plugin's daily work leaves behind beyond its main table.
var pluginLife = map[string][]string{
	"pharmacy":    {"pharmacy_controlled_products"},
	"productplus": {"pp_field_categories"},
	"documents":   {"doc_consumable", "doc_extra"},
	"recharge":    {"recharge_bill_tx", "recharge_counts"},
	"repairs":     {"repair_deposits", "repair_payouts"},
	"requests":    {"request_asks"},
	"clearance":   {"clearance_markdowns"},
}

func TestPluginLifeFilled(t *testing.T) {
	need(t)
	for _, k := range shop.Plugins {
		for _, tb := range pluginLife[k] {
			if count(t, `SELECT count(*) FROM `+tb) == 0 {
				t.Errorf("%s: nothing in %s", k, tb)
			}
		}
	}
}

// Print & Copy sells by size and type, and a job with extras rings as one itemised block.
func TestPrintAndCopyByTypeWithExtras(t *testing.T) {
	need(t)
	if !slices.Contains(shop.Plugins, "documents") {
		t.Skip("no Print & Copy in this shop")
	}
	if count(t, `SELECT count(*) FROM doc_job WHERE material IN ('Gloss', 'Matte')`) == 0 {
		t.Error("no Gloss / Matte lamination sold")
	}
	if count(t, `SELECT count(DISTINCT (sale_id, line_group)) FROM sale_items WHERE line_group_itemised`) == 0 {
		t.Error("no job sold with its extras (an itemised group)")
	}
	if count(t, `SELECT count(*) FROM sale_items si JOIN products p ON p.id = si.product_id
		WHERE si.line_group_itemised AND si.description NOT IN (SELECT name FROM doc_extra) AND si.description IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM doc_job j WHERE j.sale_item_id = si.id)`) > 0 {
		t.Error("a grouped job line has no ledger job")
	}
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

// A shop selling food or medicine has dated stock, and a few lots that really need
// looking at: some past their date, some close to it. The others have no dates.
func TestShopHasExpiryDates(t *testing.T) {
	need(t)
	lots := func(where string) int {
		return count(t, `SELECT count(*) FROM stock_batches WHERE qty_remaining > 0 AND `+where)
	}
	if !slices.Contains([]string{"grocery", "pharmacy", "mixed"}, shop.Catalogue) {
		if n := lots(`expiry_date IS NOT NULL`); n != 0 {
			t.Errorf("%s: %d dated lots", shop.Key, n)
		}
		return
	}
	if n := lots(`expiry_date >= CURRENT_DATE + 60`); n < 20 {
		t.Errorf("%s: only %d lots dated well ahead", shop.Key, n)
	}
	if n := lots(`expiry_date < CURRENT_DATE`); n < 1 || n > 8 {
		t.Errorf("%s: %d expired lots, want a few", shop.Key, n)
	}
	if n := lots(`expiry_date BETWEEN CURRENT_DATE AND CURRENT_DATE + 30`); n < 2 || n > 15 {
		t.Errorf("%s: %d lots close to expiry, want a few", shop.Key, n)
	}
}
