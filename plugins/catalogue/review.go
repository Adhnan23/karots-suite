package catalogue

import (
	"context"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// keepMine reviews the pick and keeps the product as the shop has it: the link takes the catalogue's values as
// they are now, so the same change isn't asked again, and a waiting change goes (CTL-9, REV-2).
func (s *Store) keepMine(ctx context.Context, productID int64) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE catalogue_links l SET reviewed = true,
			took = coalesce((SELECT jsonb_build_object('name', i.name, 'unit', i.unit, 'fields', i.fields)
				FROM catalogue_items i WHERE i.id = l.item_id), took)
			WHERE product_id = $1`, productID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM catalogue_reviews WHERE product_id = $1`, productID)
		return err
	})
}
