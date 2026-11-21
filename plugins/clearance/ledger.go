package clearance

import "karots-suite/internal/ledgercheck"

// Clearance's own rules the data can show (docs/rules/plugins/clearance.md).
func init() {
	// CLR-9: each markdown's shape.
	ledgercheck.Add("CLR-9", `
		SELECT 'markdown ' || product_id, format('%s %s %s', status, discount_type, discount_value)
		FROM clearance_markdowns
		WHERE status NOT IN ('approved', 'dismissed') OR discount_type <> 'percent'
		   OR (status = 'dismissed' AND discount_value <> 0)
		   OR (status = 'approved' AND (discount_value <= 0 OR discount_value >= 100))
		   OR NOT EXISTS (SELECT 1 FROM products p WHERE p.id = product_id)`)
	// CLR-9: settings in range.
	ledgercheck.Add("CLR-9", `
		SELECT 'clearance settings', format('stale %s, expiry %s, dead %s, default %s%%, margin %s%%',
		       stale_days, expiry_days, dead_months, default_percent, min_margin_percent)
		FROM clearance_settings
		WHERE stale_days < 1 OR expiry_days < 1 OR dead_months < 1
		   OR default_percent <= 0 OR default_percent >= 100 OR min_margin_percent < 0`)
	// CLR-8 / PLG-2: no markdown on a product merged away.
	ledgercheck.Add("CLR-8", `
		SELECT 'markdown ' || m.product_id, 'its product was merged into ' || p.merged_into_id
		FROM clearance_markdowns m JOIN products p ON p.id = m.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
