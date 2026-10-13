package requests

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct points requests converted to a merged duplicate at the kept product
// (core ProductMerger).
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE requests SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
