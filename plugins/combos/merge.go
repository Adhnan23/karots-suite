package combos

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// mergeProduct moves a merged duplicate's combo lines to the kept product (core
// ProductMerger). A combo holding both adds the duplicate's qty to the kept line.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`UPDATE combo_items k SET qty = k.qty + d.qty FROM combo_items d
		  WHERE d.product_id = $1 AND k.product_id = $2 AND k.combo_id = d.combo_id`,
		`DELETE FROM combo_items d USING combo_items k
		  WHERE d.product_id = $1 AND k.product_id = $2 AND k.combo_id = d.combo_id`,
		`UPDATE combo_items SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}
