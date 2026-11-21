package combos

import (
	"context"
	"encoding/json"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// recordSale writes a sale's combos inside the sale's own transaction (core's
// sale-line hook): one row per combo, worked out from the sale's own lines —
// sets from their groups, net and discount from what each line rang at — never
// from the till's word. A tag naming no combo is skipped: a sale is never stopped.
func recordSale(ctx context.Context, tx *sqlx.Tx, ev plugin.SaleLines) error {
	by := map[int64][]int64{}
	var order []int64
	for _, l := range ev.Lines {
		var d struct {
			ComboID int64 `json:"comboId"`
		}
		if json.Unmarshal(l.Data, &d) != nil || d.ComboID <= 0 {
			continue
		}
		if by[d.ComboID] == nil {
			order = append(order, d.ComboID)
		}
		by[d.ComboID] = append(by[d.ComboID], l.ItemID)
	}
	for _, id := range order {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO combo_sales (combo_id, name, sale_id, sets, net, discount, item_ids)
			SELECT c.id, c.name, $2,
			       (SELECT SUM(g.sets) FROM (SELECT DISTINCT si.line_group, si.line_group_sets AS sets FROM sale_items si
			                                 WHERE si.id = ANY($3) AND si.line_group IS NOT NULL) g),
			       l.net, l.discount, $3
			FROM combos c,
			     (SELECT SUM(si.subtotal) AS net, SUM(si.unit_price * si.quantity - si.subtotal) AS discount
			      FROM sale_items si WHERE si.id = ANY($3)) l
			WHERE c.id = $1
			ON CONFLICT (sale_id, combo_id) DO NOTHING`, id, ev.SaleID, pq.Array(by[id])); err != nil {
			return err
		}
	}
	return nil
}
