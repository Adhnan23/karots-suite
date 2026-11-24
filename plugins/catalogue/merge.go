package catalogue

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves a merged-away product's link and waiting change to the kept product when it has none of
// its own; otherwise they go (CTL-11, PLG-2).
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, table := range []string{"catalogue_links", "catalogue_reviews"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET product_id = $2 WHERE product_id = $1
			AND NOT EXISTS (SELECT 1 FROM `+table+` WHERE product_id = $2)`, from, into); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE product_id = $1`, from); err != nil {
			return err
		}
	}
	return nil
}
