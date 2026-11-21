package alternatives

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// rescaleSplit keeps a hand split adding up to 100 once a member has left it
// (removed, moved to another tier, merged away): the shares left are rescaled to
// whole percents by largest remainder and stored, so the page and the order agree
// (ALT-5, user 2026-11-21). With no share left the tier goes back to Auto. A tier
// split by what sells, or one already at 100, is left alone. Run inside the
// change's transaction, holding the tier row.
func rescaleSplit(ctx context.Context, tx *sqlx.Tx, tierID int64) error {
	var auto bool
	if err := tx.GetContext(ctx, &auto, `SELECT split_auto FROM alt_tiers WHERE id = $1 FOR UPDATE`, tierID); err != nil {
		return err
	}
	if auto {
		return nil
	}
	var rows []struct {
		ProductID int64 `db:"product_id"`
		Share     int64 `db:"share"`
	}
	if err := tx.SelectContext(ctx, &rows, `SELECT product_id, COALESCE(split_share, 0) AS share
		FROM alt_members WHERE tier_id = $1 ORDER BY product_id`, tierID); err != nil {
		return err
	}
	var sum int64
	weights := make([]int64, len(rows))
	for i, r := range rows {
		sum += r.Share
		weights[i] = r.Share
	}
	if sum == splitTotalShares {
		return nil
	}
	if sum == 0 {
		_, err := tx.ExecContext(ctx, `UPDATE alt_tiers SET split_auto = true WHERE id = $1`, tierID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE alt_members SET split_share = NULL WHERE tier_id = $1`, tierID)
		}
		return err
	}
	for i, n := range splitOrder(splitTotalShares, weights) {
		if _, err := tx.ExecContext(ctx, `UPDATE alt_members SET split_share = $2 WHERE product_id = $1`, rows[i].ProductID, n); err != nil {
			return err
		}
	}
	return nil
}

// tierOf is the tier a product sits in, 0 when none.
func tierOf(ctx context.Context, q sqlx.QueryerContext, productID int64) (int64, error) {
	var ids []int64
	err := sqlx.SelectContext(ctx, q, &ids, `SELECT tier_id FROM alt_members WHERE product_id = $1`, productID)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	return ids[0], nil
}
