package aicatalog

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct: the kept product's learnt item wins; the duplicate's alone moves over.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM aicatalog_items WHERE product_id = $1 AND EXISTS (SELECT 1 FROM aicatalog_items WHERE product_id = $2)`,
		`UPDATE aicatalog_items SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
