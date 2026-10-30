package pocketcashier

import (
	"context"

	"karots-suite/internal/features/sales"
)

// lookalikeWindow is how close in time two sales must be to look like one
// sale rung twice (on the PC and on the phone during a cut).
const lookalikeWindow = "10 minutes"

// flagLookalike flags a replayed sale when another sale with the same total
// and the same items (product and quantity) was rung within the window. Both
// are kept; an admin removes one with a return if it really was rung twice.
func (h handlers) flagLookalike(ctx context.Context, d *sales.Detail) {
	var receipt string
	err := h.p.core.DB.GetContext(ctx, &receipt, `
		WITH me AS (SELECT id, total, COALESCE(rung_at, created_at) AS t FROM sales WHERE id = $1),
		     mine AS (SELECT product_id, sum(quantity) AS q FROM sale_items WHERE sale_id = $1 GROUP BY product_id)
		SELECT s.receipt_no FROM sales s, me
		WHERE s.id <> me.id AND s.total = me.total
		  AND COALESCE(s.rung_at, s.created_at) BETWEEN me.t - $2::interval AND me.t + $2::interval
		  AND NOT EXISTS ((SELECT product_id, sum(quantity) FROM sale_items WHERE sale_id = s.id GROUP BY product_id) EXCEPT (SELECT * FROM mine))
		  AND NOT EXISTS ((SELECT * FROM mine) EXCEPT (SELECT product_id, sum(quantity) FROM sale_items WHERE sale_id = s.id GROUP BY product_id))
		ORDER BY abs(extract(epoch FROM COALESCE(s.rung_at, s.created_at) - me.t)) LIMIT 1`, d.Sale.ID, lookalikeWindow)
	if err != nil {
		return // none (or a read error: the sale stands either way)
	}
	_ = sales.AddFlag(ctx, h.p.core.DB, d.Sale.ID, sales.FlagLookalike, "looks like "+receipt+" — the same items and total a few minutes apart")
}
