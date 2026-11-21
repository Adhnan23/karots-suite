package documents

import "karots-suite/internal/ledgercheck"

// Print & Copy's own rules the data can show (docs/rules/plugins/documents.md).
func init() {
	// DOC-1: a print service is one core service product, kind and category known.
	ledgercheck.Add("DOC-1", `
		SELECT 'print service ' || s.id, format('%s / %s on product %s (service %s, %s print services)', s.kind, s.category,
		       s.product_id, p.is_service, (SELECT COUNT(*) FROM doc_service x WHERE x.product_id = s.product_id))
		FROM doc_service s JOIN products p ON p.id = s.product_id
		WHERE NOT p.is_service OR s.kind NOT IN ('metered', 'custom')
		   OR s.category NOT IN ('copy', 'print', 'laminate', 'bind', 'other')
		   OR (SELECT COUNT(*) FROM doc_service x WHERE x.product_id = s.product_id) > 1`)
	// DOC-3: the grid's prices, each cell once.
	ledgercheck.Add("DOC-3", `
		SELECT 'print price ' || MIN(id), format('service %s %s/%s colour %s 2-side %s from %s: %s rows, price %s',
		       service_id, size, material, color, double_side, min_qty, COUNT(*), MIN(unit_price))
		FROM doc_price GROUP BY service_id, size, material, color, double_side, min_qty
		HAVING COUNT(*) > 1 OR MIN(unit_price) < 0 OR min_qty < 1`)
	// DOC-3: paper is a stocked product at a qty above 0 per sheet.
	ledgercheck.Add("DOC-3", `
		SELECT 'print paper ' || c.id, format('product %s (service %s), %s a sheet', c.product_id, p.is_service, c.qty_per_unit)
		FROM doc_consumable c JOIN products p ON p.id = c.product_id
		WHERE c.qty_per_unit <= 0 OR p.is_service`)
	// DOC-3: an extra's price, and its stock a stocked product.
	ledgercheck.Add("DOC-3", `
		SELECT 'print extra ' || e.id, format('%s: price %s, qty %s of product %s', e.name, e.price, e.qty, e.product_id)
		FROM doc_extra e LEFT JOIN products p ON p.id = e.product_id
		WHERE e.price < 0 OR e.qty <= 0 OR btrim(e.name) = '' OR p.is_service`)
	// DOC-6: a job is its sale line's own qty, price and total, on that line's sale and
	// the service's product (or one merged into it).
	ledgercheck.Add("DOC-6", `
		SELECT 'print job ' || j.id, format('sale %s line %s: job %s × %s = %s; line on sale %s, %s × %s = %s, product %s',
		       j.sale_id, j.sale_item_id, j.qty, j.unit_price, j.line_total, si.sale_id, si.quantity, si.unit_price, si.subtotal, si.product_id)
		FROM doc_job j JOIN sale_items si ON si.id = j.sale_item_id
		LEFT JOIN doc_service sv ON sv.id = j.service_id
		LEFT JOIN products lp ON lp.id = si.product_id
		WHERE j.sale_id IS DISTINCT FROM si.sale_id OR j.qty <> si.quantity OR j.unit_price <> si.unit_price
		   OR j.line_total <> si.subtotal
		   OR (sv.id IS NOT NULL AND si.product_id <> sv.product_id AND lp.merged_into_id IS DISTINCT FROM sv.product_id)`)
	// DOC-9 / PLG-2: nothing names a product merged away.
	ledgercheck.Add("DOC-9", `
		SELECT x.what, 'its product was merged into ' || p.merged_into_id
		FROM (SELECT 'print paper ' || id AS what, product_id FROM doc_consumable
		      UNION ALL SELECT 'print extra ' || id, product_id FROM doc_extra WHERE product_id IS NOT NULL
		      UNION ALL SELECT 'print service ' || id, product_id FROM doc_service) x
		JOIN products p ON p.id = x.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
