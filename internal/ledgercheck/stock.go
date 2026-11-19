package ledgercheck

// Area 4: stock, lots, counts, write-offs (docs/rules/stock.md). On hand = Σ
// movements is SALE-20's check (STK-1).
func init() {
	checks = append(checks,
		// STK-2: a product's lots add up to its on hand.
		check{"STK-2", `
			SELECT 'product ' || st.product_id, format('on hand %s, lots %s', st.quantity, COALESCE(b.q, 0))
			FROM stock st LEFT JOIN (SELECT product_id, SUM(qty_remaining) AS q FROM stock_batches GROUP BY product_id) b
			  ON b.product_id = st.product_id
			WHERE st.quantity <> COALESCE(b.q, 0)`},

		// STK-3: never below zero.
		check{"STK-3", `
			SELECT 'product ' || product_id, format('on hand %s', quantity) FROM stock WHERE quantity < 0`},

		// STK-7: a non-sale movement that moved units records their worth, never below
		// zero, and above zero when the product has a cost (a blank-cost lot is worth the
		// product's cost). A till quick-add has no cost yet when it's added (the review
		// queue fills it in later), so its opening was rightly worth 0.
		check{"STK-7", `
			SELECT 'movement ' || m.id, format('%s %s worth %s, product cost %s', m.type, m.quantity, m.cost, p.cost_price)
			FROM stock_movements m JOIN products p ON p.id = m.product_id
			WHERE m.cost < 0
			   OR (m.type IN ('adjust', 'damage', 'own_use', 'staff', 'warranty_replacement', 'to_shelf')
			       AND m.quantity <> 0 AND m.cost = 0 AND p.cost_price > 0
			       AND m.note IS DISTINCT FROM 'quick-add opening (count pending)')`},

		// STK-17: a pack holds a whole number of units.
		check{"STK-17", `
			SELECT 'pack ' || id, format('size %s', size) FROM product_packs WHERE size <> trunc(size)`},

		// STK-18: a whole-unit product never holds a fraction, unless a service's recipe
		// uses part of one (user, 2026-11-19).
		check{"STK-18", `
			SELECT 'product ' || p.id, format('on hand %s %s', st.quantity, u.abbreviation)
			FROM products p JOIN units u ON u.id = p.unit_id JOIN stock st ON st.product_id = p.id
			WHERE NOT u.allow_decimal AND st.quantity <> trunc(st.quantity)
			  AND NOT EXISTS (SELECT 1 FROM product_recipes r WHERE r.component_product_id = p.id AND NOT r.whole_units)`},
	)
}
