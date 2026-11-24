package catalogue

import "karots-suite/internal/ledgercheck"

// The Catalogue's own rules the data can show (docs/rules/plugins/catalogue.md).
func init() {
	// CTL-2: the copy holds as many products as its version says.
	ledgercheck.Add("CTL-2", `
		SELECT 'catalogue version ' || s.version, format('%s products, the version has %s', (SELECT count(*) FROM catalogue_items), s.count)
		FROM catalogue_state s
		WHERE s.version > 0 AND s.count <> (SELECT count(*) FROM catalogue_items)`)
	// CTL-7, CTL-11: no link or waiting change names a missing or merged-away product.
	ledgercheck.Add("CTL-7", `
		SELECT 'catalogue link for product ' || l.product_id, CASE WHEN p.id IS NULL THEN 'no such product' ELSE 'merged into ' || p.merged_into_id END
		FROM catalogue_links l LEFT JOIN products p ON p.id = l.product_id
		WHERE p.id IS NULL OR p.merged_into_id IS NOT NULL
		UNION ALL
		SELECT 'catalogue change for product ' || r.product_id, CASE WHEN p.id IS NULL THEN 'no such product' ELSE 'merged into ' || p.merged_into_id END
		FROM catalogue_reviews r LEFT JOIN products p ON p.id = r.product_id
		WHERE p.id IS NULL OR p.merged_into_id IS NOT NULL`)
}
