package ledgercheck

// Area 6: recovery and losses — warranty replacements and what a supplier makes
// good on a loss (docs/rules/recovery.md).
func init() {
	checks = append(checks,
		// REC-1: one recovery per loss.
		check{"REC-1", `
			SELECT source_type || ' ' || source_id, format('%s recoveries of one loss', count(*))
			FROM loss_recoveries
			GROUP BY source_type, source_id
			HAVING count(*) > 1`},

		// REC-1: a recovery's loss is its source's: a damage write-off's worth and units,
		// or a replaced warranty unit (1 unit) worth its claim's movement.
		check{"REC-1", `
			SELECT 'recovery ' || lr.id, format('%s: loss %s × %s, source %s × %s', lr.source_type, lr.loss_value, lr.quantity,
			       COALESCE(src.worth::text, 'missing'), COALESCE(src.qty::text, 'missing'))
			FROM loss_recoveries lr
			LEFT JOIN LATERAL (
				SELECT m.cost AS worth, ABS(m.quantity) AS qty, m.product_id
				FROM stock_movements m
				WHERE lr.source_type = 'damage' AND m.id = lr.source_id AND m.type = 'damage'
			  UNION ALL
				SELECT m.cost, 1, wu.product_id
				FROM warranty_units wu
				JOIN warranty_claims c ON c.unit_id = wu.id AND c.resolution = 'replaced'
				JOIN stock_movements m ON m.type = 'warranty_replacement' AND m.reference_type = 'warranty' AND m.reference_id = c.id
				WHERE lr.source_type = 'warranty' AND wu.id = lr.source_id AND wu.status = 'replaced'
			) src ON true
			WHERE src.worth IS NULL OR src.worth <> lr.loss_value OR src.qty <> lr.quantity OR src.product_id <> lr.product_id`},

		// REC-5: what a recovery holds agrees with its outcome.
		check{"REC-5", `
			SELECT 'recovery ' || id, format('%s: loss %s, recovered %s, extra %s, left for bills %s, supplier %s',
			       outcome, loss_value, recovered_value, extra_charge, credit_unapplied, COALESCE(supplier_id::text, 'none'))
			FROM loss_recoveries
			WHERE outcome NOT IN ('replacement', 'paid', 'written_off')
			   OR (outcome = 'replacement' AND recovered_value <> loss_value)
			   OR (outcome = 'paid' AND (supplier_id IS NULL OR recovered_value <= 0 OR extra_charge <> 0))
			   OR (outcome = 'written_off' AND (recovered_value <> 0 OR extra_charge <> 0))
			   OR credit_unapplied < 0 OR credit_unapplied > recovered_value OR (outcome <> 'paid' AND credit_unapplied <> 0)
			   OR extra_charge < 0 OR (extra_charge > 0 AND supplier_id IS NULL)`},

		// WAR-3: every replacement claim handed out exactly one unit from stock (−1, or 0
		// for a frozen product), and its unit is replaced; every warranty movement has its claim.
		check{"WAR-3", `
			SELECT 'claim ' || c.id, format('%s replacement movements (quantities %s), unit %s', m.n, COALESCE(m.qs, '-'), wu.status)
			FROM warranty_claims c
			JOIN warranty_units wu ON wu.id = c.unit_id
			CROSS JOIN LATERAL (SELECT count(*) AS n, string_agg(quantity::text, ',') AS qs, bool_and(quantity IN (-1, 0)) AS ok
			                    FROM stock_movements WHERE type = 'warranty_replacement' AND reference_type = 'warranty' AND reference_id = c.id) m
			WHERE c.resolution = 'replaced' AND (m.n <> 1 OR NOT m.ok OR wu.status <> 'replaced')
			UNION ALL
			SELECT 'movement ' || m.id, 'a warranty replacement with no claim'
			FROM stock_movements m
			WHERE m.type = 'warranty_replacement'
			  AND NOT EXISTS (SELECT 1 FROM warranty_claims c WHERE m.reference_type = 'warranty' AND c.id = m.reference_id AND c.resolution = 'replaced')`},

		// WAR-2: a serial replacement continues the old unit: same product, customer and last day.
		check{"WAR-2", `
			SELECT 'unit ' || o.id, format('replaced by unit %s: product %s→%s, customer %s→%s, until %s→%s',
			       n.id, o.product_id, n.product_id, o.customer_id, n.customer_id, o.warranty_until, n.warranty_until)
			FROM warranty_units o
			JOIN warranty_units n ON n.id = o.replaced_by_unit_id
			WHERE n.product_id <> o.product_id OR n.customer_id IS DISTINCT FROM o.customer_id OR n.warranty_until <> o.warranty_until
			   OR n.source <> 'replacement'`},

		// WAR-4: a sale's by-receipt replacements of a product never pass the units sold
		// less the units returned, and a void sale has none.
		check{"WAR-4", `
			SELECT 'sale ' || wu.sale_id || ' product ' || wu.product_id,
			       format('%s replaced, %s sold, %s returned, sale %s', count(*), COALESCE(si.sold, 0), COALESCE(si.back, 0), s.status)
			FROM warranty_units wu
			JOIN sales s ON s.id = wu.sale_id
			LEFT JOIN LATERAL (SELECT SUM(quantity) AS sold, SUM(returned_qty) AS back
			                   FROM sale_items WHERE sale_id = wu.sale_id AND product_id = wu.product_id) si ON true
			WHERE wu.serial_no IS NULL AND wu.source = 'sale' AND wu.status = 'replaced'
			GROUP BY wu.sale_id, wu.product_id, si.sold, si.back, s.status
			HAVING count(*) > COALESCE(si.sold, 0) - COALESCE(si.back, 0) OR s.status = 'void'`},
	)
}
