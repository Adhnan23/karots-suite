package clearance

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves a merged duplicate's markdown to the kept product (core
// ProductMerger). The kept product's own markdown, if any, wins.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM clearance_markdowns WHERE product_id = $1 AND EXISTS (SELECT 1 FROM clearance_markdowns WHERE product_id = $2)`,
		`UPDATE clearance_markdowns SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
