package aicatalog

import "karots-suite/internal/ledgercheck"

// AI Assistant's own rules the data can show (docs/rules/plugins/aicatalog.md).
func init() {
	// AIC-5 / PLG-2: nothing it learnt names a product merged away.
	ledgercheck.Add("AIC-5", `
		SELECT 'learnt item for product ' || i.product_id, 'its product was merged into ' || p.merged_into_id
		FROM aicatalog_items i JOIN products p ON p.id = i.product_id WHERE p.merged_into_id IS NOT NULL
		UNION ALL
		SELECT format('job %s item for product %s', j.job, j.product_id), 'its product was merged into ' || p.merged_into_id
		FROM aicatalog_job_items j JOIN products p ON p.id = j.product_id WHERE p.merged_into_id IS NOT NULL`)
}
