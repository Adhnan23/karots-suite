package recharge

import "karots-suite/internal/ledgercheck"

// Reload & Bills' own rules the data can show (docs/rules/plugins/recharge.md).
func init() {
	// RCH-3: each recharge line is booked once.
	ledgercheck.Add("RCH-3", `
		SELECT 'sale line ' || sale_item_id, COUNT(*) || ' bookings'
		FROM (SELECT sale_item_id FROM recharge_transactions
		      WHERE sale_item_id IS NOT NULL AND parent_id IS NULL AND type IN ('reload','billpay','deposit','cashout')
		      UNION ALL SELECT sale_item_id FROM recharge_bill_tx WHERE sale_item_id IS NOT NULL) x
		GROUP BY sale_item_id HAVING COUNT(*) > 1`)
	// RCH-3: a reload, bill, deposit or cash out moves the float by its line's face
	// value (out; a cash out in). A package uses its own float cost.
	ledgercheck.Add("RCH-3", `
		SELECT 'float row ' || t.id, format('%s of %s, float %s; line %s × %s', t.type, t.amount, t.float_delta, si.quantity, si.unit_price)
		FROM recharge_transactions t JOIN sale_items si ON si.id = t.sale_item_id
		WHERE t.parent_id IS NULL AND t.package_id IS NULL AND t.type IN ('reload','billpay','deposit','cashout')
		  AND (t.amount <> abs(si.quantity * si.unit_price)
		       OR t.float_delta <> CASE WHEN t.type = 'cashout' THEN t.amount ELSE -t.amount END)`)
	// RCH-3: a commission row hangs off a move of its own device and sale line.
	ledgercheck.Add("RCH-3", `
		SELECT 'commission row ' || c.id, format('on device %s, line %s; its move %s is on device %s, line %s',
		       c.device_id, c.sale_item_id, m.id, m.device_id, m.sale_item_id)
		FROM recharge_transactions c JOIN recharge_transactions m ON m.id = c.parent_id
		WHERE c.type = 'commission' AND (c.device_id <> m.device_id OR c.sale_item_id IS DISTINCT FROM m.sale_item_id)`)
	// RCH-5: a bank leg's bill row is its line's face value; never more given back than paid.
	ledgercheck.Add("RCH-5", `
		SELECT 'bank bill ' || b.id, format('%s of %s, %s given back; line %s × %s', b.type, b.amount, b.returned, si.quantity, si.unit_price)
		FROM recharge_bill_tx b JOIN sale_items si ON si.id = b.sale_item_id
		WHERE b.amount <> abs(si.quantity * si.unit_price) OR b.returned < 0 OR b.returned > b.amount`)
	// RCH-6: a line's returns never put back more float, or take back more
	// commission, than the line moved.
	ledgercheck.Add("RCH-6", `
		SELECT 'float row ' || m.id, format('%s out, %s back; commission %s, %s taken back', -m.float_delta,
		       COALESCE(SUM(r.float_delta) FILTER (WHERE r.type IN ('reversal','reversal_lost')), 0),
		       COALESCE(SUM(r.float_delta) FILTER (WHERE r.type = 'commission' AND r.float_delta > 0), 0),
		       -COALESCE(SUM(r.float_delta) FILTER (WHERE r.type = 'commission' AND r.float_delta < 0), 0))
		FROM recharge_transactions m JOIN recharge_transactions r ON r.parent_id = m.id
		WHERE m.parent_id IS NULL AND m.type IN ('reload','billpay','deposit')
		GROUP BY m.id, m.float_delta
		HAVING COALESCE(SUM(r.float_delta) FILTER (WHERE r.type IN ('reversal','reversal_lost')), 0) > -m.float_delta
		    OR -COALESCE(SUM(r.float_delta) FILTER (WHERE r.type = 'commission' AND r.float_delta < 0), 0)
		       > COALESCE(SUM(r.float_delta) FILTER (WHERE r.type = 'commission' AND r.float_delta > 0), 0)`)
	// RCH-8: a count books exactly counted − expected, and expected is the float
	// booked before it (devices with old sessions read through their last close: skipped).
	ledgercheck.Add("RCH-8", `
		SELECT 'count ' || k.id, format('counted %s, expected %s, booked %s; float before it %s', k.counted, k.expected,
		       COALESCE(t.float_delta, 0), (SELECT COALESCE(SUM(b.float_delta), 0) FROM recharge_transactions b
		                                    WHERE b.device_id = k.device_id AND b.id < t.id))
		FROM recharge_counts k LEFT JOIN recharge_transactions t ON t.count_id = k.id
		WHERE COALESCE(t.float_delta, 0) <> k.counted - k.expected
		   OR (t.id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM recharge_device_sessions s WHERE s.device_id = k.device_id)
		       AND k.expected <> (SELECT COALESCE(SUM(b.float_delta), 0) FROM recharge_transactions b
		                          WHERE b.device_id = k.device_id AND b.id < t.id))`)
	// RCH-11 / PLG-2: nothing names a product merged away.
	ledgercheck.Add("RCH-11", `
		SELECT x.what, 'its product was merged into ' || p.merged_into_id
		FROM (SELECT 'carrier ' || name AS what, product_id FROM recharge_carriers
		      UNION ALL SELECT 'own product ' || key, product_id FROM recharge_config) x
		JOIN products p ON p.id = x.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
