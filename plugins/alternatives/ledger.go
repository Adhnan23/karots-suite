package alternatives

import "karots-suite/internal/ledgercheck"

// Alternatives' own rules the data can show (docs/rules/plugins/alternatives.md).
func init() {
	// ALT-5: a tier split by hand has shares adding up to 100.
	ledgercheck.Add("ALT-5", `
		SELECT 'tier ' || t.id, format('split by hand, shares add up to %s', COALESCE(sum(m.split_share), 0))
		FROM alt_tiers t LEFT JOIN alt_members m ON m.tier_id = t.id
		WHERE NOT t.split_auto
		GROUP BY t.id HAVING COALESCE(sum(m.split_share), 0) <> 100`)
	// ALT-2 / ALT-7 / PLG-2: a member is a real product, not a service, not merged away.
	ledgercheck.Add("ALT-2", `
		SELECT 'member ' || m.product_id, CASE WHEN p.id IS NULL THEN 'no such product'
		       WHEN p.merged_into_id IS NOT NULL THEN 'merged into ' || p.merged_into_id ELSE 'a service' END
		FROM alt_members m LEFT JOIN products p ON p.id = m.product_id
		WHERE p.id IS NULL OR p.merged_into_id IS NOT NULL OR p.is_service`)
}
