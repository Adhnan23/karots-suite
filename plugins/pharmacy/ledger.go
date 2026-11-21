package pharmacy

import "karots-suite/internal/ledgercheck"

// Pharmacy's own rules the data can show (docs/rules/plugins/pharmacy.md).
func init() {
	// PHA-12: dispense rows above 0, a script line of the dispense's own patient.
	ledgercheck.Add("PHA-12", `
		SELECT 'dispense item ' || i.id, format('dispense %s for patient %s: %s of product %s on script line %s (patient %s)',
		       d.id, d.patient_id, i.quantity, i.product_id, i.prescription_item_id, rx.patient_id)
		FROM pharmacy_dispense_items i JOIN pharmacy_dispenses d ON d.id = i.dispense_id
		LEFT JOIN pharmacy_prescription_items ri ON ri.id = i.prescription_item_id
		LEFT JOIN pharmacy_prescriptions rx ON rx.id = ri.prescription_id
		WHERE i.quantity <= 0 OR rx.patient_id <> d.patient_id`)
	// PHA-12: one dispense per sale.
	ledgercheck.Add("PHA-12", `
		SELECT 'sale ' || sale_id, COUNT(*) || ' dispenses' FROM pharmacy_dispenses
		WHERE sale_id IS NOT NULL GROUP BY sale_id HAVING COUNT(*) > 1`)
	// PHA-5: a dispense row never gives more of a product than its sale sold.
	ledgercheck.Add("PHA-5", `
		SELECT 'dispense item ' || i.id, format('sale %s: %s of product %s given, %s sold', d.sale_id, i.quantity, i.product_id,
		       COALESCE((SELECT SUM(si.quantity) FROM sale_items si WHERE si.sale_id = d.sale_id AND si.product_id = i.product_id), 0))
		FROM pharmacy_dispense_items i JOIN pharmacy_dispenses d ON d.id = i.dispense_id
		WHERE d.sale_id IS NOT NULL AND i.product_id IS NOT NULL
		  AND i.quantity > COALESCE((SELECT SUM(si.quantity) FROM sale_items si WHERE si.sale_id = d.sale_id AND si.product_id = i.product_id), 0)`)
	// PHA-8: a controlled drug is always Rx-only.
	ledgercheck.Add("PHA-8", `
		SELECT 'controlled product ' || c.product_id, 'not marked prescription-only'
		FROM pharmacy_controlled_products c LEFT JOIN pharmacy_medicines m ON m.product_id = c.product_id
		WHERE NOT COALESCE(m.rx_only, false)`)
	// PHA-10 / PLG-2: nothing names a product merged away.
	ledgercheck.Add("PHA-10", `
		SELECT x.what, 'its product was merged into ' || p.merged_into_id
		FROM (SELECT 'script line ' || id AS what, product_id FROM pharmacy_prescription_items
		      UNION ALL SELECT 'dispense item ' || id, product_id FROM pharmacy_dispense_items
		      UNION ALL SELECT 'controlled mark', product_id FROM pharmacy_controlled_products
		      UNION ALL SELECT 'medicine marks', product_id FROM pharmacy_medicines
		      UNION ALL SELECT 'old register row ' || id, product_id FROM pharmacy_cd_ledger) x
		JOIN products p ON p.id = x.product_id
		WHERE p.merged_into_id IS NOT NULL`)
}
