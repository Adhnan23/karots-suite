package combos

import "karots-suite/internal/ledgercheck"

// Combos' own rules the data can show (docs/rules/plugins/combos.md).
func init() {
	// CMB-10: a combo's deal and dates.
	ledgercheck.Add("CMB-10", `
		SELECT 'combo ' || id, format('%s %s, %s → %s', discount_type, discount_value, starts_on, ends_on)
		FROM combos
		WHERE discount_type NOT IN ('percent', 'fixed', 'price') OR discount_value < 0
		   OR (discount_type = 'percent' AND discount_value > 100) OR (discount_type = 'price' AND discount_value <= 0)
		   OR ends_on < starts_on`)
	// CMB-10: members once per combo, qty above 0.
	ledgercheck.Add("CMB-10", `
		SELECT 'combo ' || combo_id || ' member ' || product_id, format('qty %s, %s times', MIN(qty), COUNT(*))
		FROM combo_items GROUP BY combo_id, product_id HAVING COUNT(*) > 1 OR MIN(qty) <= 0`)
	// CMB-10: one settings row; sold rows' shape.
	ledgercheck.Add("CMB-10", `
		SELECT 'combo settings', COUNT(*) || ' rows' FROM combos_settings HAVING COUNT(*) > 1`)
	ledgercheck.Add("CMB-10", `
		SELECT 'combo sale ' || id, format('sale %s: %s sets, net %s, discount %s', sale_id, sets, net, discount)
		FROM combo_sales cs
		WHERE sets <= 0 OR net < 0 OR discount < 0 OR NOT EXISTS (SELECT 1 FROM sales s WHERE s.id = cs.sale_id)`)
	// CMB-5: a sold row is what its sale lines came to (rows from before the hook name none).
	ledgercheck.Add("CMB-5", `
		SELECT 'combo sale ' || cs.id, format('stored %s sets / %s / %s; its lines %s / %s / %s on %s of %s lines',
		       cs.sets, cs.net, cs.discount, g.sets, l.net, l.discount, l.n, cardinality(cs.item_ids))
		FROM combo_sales cs
		CROSS JOIN LATERAL (SELECT COUNT(*) AS n, SUM(si.subtotal) AS net, SUM(si.unit_price * si.quantity - si.subtotal) AS discount
		                    FROM sale_items si WHERE si.id = ANY(cs.item_ids) AND si.sale_id = cs.sale_id) l
		CROSS JOIN LATERAL (SELECT SUM(x.sets) AS sets FROM (SELECT DISTINCT si.line_group, si.line_group_sets AS sets
		                    FROM sale_items si WHERE si.id = ANY(cs.item_ids) AND si.sale_id = cs.sale_id) x) g
		WHERE cardinality(cs.item_ids) > 0
		  AND (l.n <> cardinality(cs.item_ids) OR l.net IS DISTINCT FROM cs.net OR l.discount IS DISTINCT FROM cs.discount
		       OR g.sets IS DISTINCT FROM cs.sets)`)
	// CMB-9 / PLG-2: no member is a product merged away.
	ledgercheck.Add("CMB-9", `
		SELECT 'combo ' || ci.combo_id || ' member ' || ci.product_id, 'its product was merged into ' || p.merged_into_id
		FROM combo_items ci JOIN products p ON p.id = ci.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
