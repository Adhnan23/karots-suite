package aicatalog

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct: the kept product's learnt item wins; the duplicate's alone moves over.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE aicatalog_items k SET old_names = trim(k.old_names || ' ' || d.old_names)
		 FROM aicatalog_items d WHERE d.product_id = $1 AND k.product_id = $2 AND d.old_names <> ''`,
		`DELETE FROM aicatalog_items WHERE product_id = $1 AND EXISTS (SELECT 1 FROM aicatalog_items WHERE product_id = $2)`,
		`UPDATE aicatalog_items SET product_id = $2 WHERE product_id = $1`,
		`DELETE FROM aicatalog_job_items d WHERE d.product_id = $1 AND EXISTS (SELECT 1 FROM aicatalog_job_items k WHERE k.job = d.job AND k.product_id = $2)`,
		`UPDATE aicatalog_job_items SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
