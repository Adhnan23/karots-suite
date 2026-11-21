package repairs

import "karots-suite/internal/ledgercheck"

// Repairs' own rules the data can show (docs/rules/plugins/repairs.md).
func init() {
	// REP-1: a closed job has its time stamped, and only a closed one.
	ledgercheck.Add("REP-1", `
		SELECT 'repair ' || ticket_no, format('status %s, collected at %s, cancelled at %s', status, collected_at, cancelled_at)
		FROM repair_jobs
		WHERE (status = 'collected') <> (collected_at IS NOT NULL) OR (status = 'cancelled') <> (cancelled_at IS NOT NULL)`)
	// REP-5: a collected job names its pickup sale, and the sale exists.
	ledgercheck.Add("REP-5", `
		SELECT 'repair ' || j.ticket_no, format('collected on sale %s, which isn''t there', j.sale_id)
		FROM repair_jobs j LEFT JOIN sales s ON s.id = j.sale_id
		WHERE j.status = 'collected' AND s.id IS NULL`)
	// REP-2: a part is a stocked product, qty above 0.
	ledgercheck.Add("REP-2", `
		SELECT 'repair part ' || rp.id, format('%s of product %s (service %s)', rp.qty, rp.product_id, p.is_service)
		FROM repair_parts rp LEFT JOIN products p ON p.id = rp.product_id
		WHERE rp.qty <= 0 OR COALESCE(p.is_service, true)`)
	// REP-7: a deposit is a payment of the job's own customer.
	ledgercheck.Add("REP-7", `
		SELECT 'repair ' || j.ticket_no, format('deposit payment %s is customer %s''s, the job is %s''s', cp.id, cp.customer_id, j.customer_id)
		FROM repair_deposits d JOIN repair_jobs j ON j.id = d.job_id JOIN customer_payments cp ON cp.id = d.payment_id
		WHERE j.customer_id IS DISTINCT FROM cp.customer_id`)
	// REP-7: an old held deposit is never spent or handed back past what was taken.
	ledgercheck.Add("REP-7", `
		SELECT 'repair ' || j.ticket_no, format('old deposit held %s', SUM(CASE WHEN p.kind = 'deposit' THEN p.amount ELSE -p.amount END))
		FROM repair_payments p JOIN repair_jobs j ON j.id = p.job_id
		GROUP BY j.id, j.ticket_no HAVING SUM(CASE WHEN p.kind = 'deposit' THEN p.amount ELSE -p.amount END) < 0`)
	// REP-10 / PLG-2: nothing names a product merged away.
	ledgercheck.Add("REP-10", `
		SELECT x.what, 'its product was merged into ' || p.merged_into_id
		FROM (SELECT 'repair part ' || id AS what, product_id FROM repair_parts
		      UNION ALL SELECT 'repair labour product', labour_product_id FROM repair_config) x
		JOIN products p ON p.id = x.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
