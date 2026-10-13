package alternatives

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves a merged duplicate's group membership to the kept product
// (core ProductMerger). A product sits in one tier only, so when the kept one is
// already a member the duplicate's membership just goes.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM alt_members WHERE product_id = $1 AND EXISTS (SELECT 1 FROM alt_members WHERE product_id = $2)`,
		`UPDATE alt_members SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
