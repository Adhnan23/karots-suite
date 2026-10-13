package productplus

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves a merged duplicate's field values to the kept product (core
// ProductMerger). Where both have a value for a field, the kept product's wins.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM pp_values d USING pp_values k
		  WHERE d.product_id = $1 AND k.product_id = $2 AND k.field_id = d.field_id`,
		`UPDATE pp_values SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
