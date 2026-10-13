package documents

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves Print & Copy rows from a merged duplicate to the kept product
// (core ProductMerger). Paper: a size a service already takes from the kept product
// keeps that row. Two print services become one: the duplicate's jobs move to the
// kept product's service and its own link (with its price grid) goes; a duplicate
// that was the only print service hands its link over.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM doc_consumable d USING doc_consumable k
		  WHERE d.product_id = $1 AND k.product_id = $2 AND k.service_id = d.service_id AND k.size IS NOT DISTINCT FROM d.size`,
		`UPDATE doc_consumable SET product_id = $2 WHERE product_id = $1`,
		`UPDATE doc_job j SET service_id = (SELECT MIN(id) FROM doc_service WHERE product_id = $2)
		  WHERE j.service_id IN (SELECT id FROM doc_service WHERE product_id = $1)
		    AND EXISTS (SELECT 1 FROM doc_service WHERE product_id = $2)`,
		`DELETE FROM doc_service WHERE product_id = $1 AND EXISTS (SELECT 1 FROM doc_service WHERE product_id = $2)`,
		`UPDATE doc_service SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
