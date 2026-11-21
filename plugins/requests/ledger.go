package requests

import "karots-suite/internal/ledgercheck"

// Customer requests' own rules the data can show (docs/rules/plugins/requests.md).
func init() {
	// REQ-9: a request's shape — its status, product and reason agree.
	ledgercheck.Add("REQ-9", `
		SELECT 'request ' || r.id, format('%s, product %s, reason %L', r.status, COALESCE(r.product_id::text, 'none'), r.reason)
		FROM requests r
		WHERE r.status NOT IN ('open', 'converted', 'declined')
		   OR (r.status = 'converted' AND NOT EXISTS (SELECT 1 FROM products p WHERE p.id = r.product_id))
		   OR (r.status <> 'converted' AND r.product_id IS NOT NULL)
		   OR (r.status = 'declined' AND btrim(r.reason) = '')
		   OR btrim(r.note) = ''`)
	// REQ-7 / PLG-2: no request names a product merged away.
	ledgercheck.Add("REQ-7", `
		SELECT 'request ' || r.id, 'its product was merged into ' || p.merged_into_id
		FROM requests r JOIN products p ON p.id = r.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
