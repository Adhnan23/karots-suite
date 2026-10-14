package pharmacy

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct re-points a merged duplicate's pharmacy rows at the kept product
// (core ProductMerger). The controlled mark and the Rx-only flag are one row per
// product: the kept one's wins, else the duplicate's moves over.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE pharmacy_prescription_items SET product_id = $2 WHERE product_id = $1`,
		`UPDATE pharmacy_dispense_items SET product_id = $2 WHERE product_id = $1`,
		`UPDATE pharmacy_cd_ledger SET product_id = $2 WHERE product_id = $1`,
		`DELETE FROM pharmacy_controlled_products WHERE product_id = $1 AND EXISTS (SELECT 1 FROM pharmacy_controlled_products WHERE product_id = $2)`,
		`UPDATE pharmacy_controlled_products SET product_id = $2 WHERE product_id = $1`,
		`DELETE FROM pharmacy_medicines WHERE product_id = $1 AND EXISTS (SELECT 1 FROM pharmacy_medicines WHERE product_id = $2)`,
		`UPDATE pharmacy_medicines SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
