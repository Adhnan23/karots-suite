package documents

import (
	"context"
	"encoding/json"
	"strings"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// lineJob is what the till tags a job line with (web/src/plugins/documents
// till.ts LineJob); an extra's line carries extraOf instead and is no job.
type lineJob struct {
	ServiceID int64   `json:"serviceId"`
	Size      *string `json:"size"`
	Material  string  `json:"material"`
	Color     *bool   `json:"color"`
	Double    *bool   `json:"double"`
	Sheets    *int    `json:"sheets"`
}

// recordSale writes one job per job line of a sale, inside the sale's own
// transaction (core's sale-line hook, DOC-6): qty, price and total copied from
// the line itself, the options from its tag. A tag naming no print service of
// the line's product is skipped — a sale is never stopped.
func recordSale(ctx context.Context, tx *sqlx.Tx, ev plugin.SaleLines) error {
	for _, l := range ev.Lines {
		var j lineJob
		if json.Unmarshal(l.Data, &j) != nil || j.ServiceID <= 0 {
			continue
		}
		if j.Size != nil && *j.Size == "" {
			j.Size = nil
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO doc_job (sale_id, sale_item_id, service_id, description, qty, unit_price, line_total,
			                     consumable_cost, kind, size, color, double_side, sheets, material)
			SELECT si.sale_id, si.id, sv.id, COALESCE(si.description, ''), si.quantity, si.unit_price, si.subtotal,
			       ROUND(si.cost_price * si.quantity, 2), 'sale', LEFT($3, 20), $4, $5, $6, LEFT($7, 30)
			FROM sale_items si JOIN doc_service sv ON sv.id = $2 AND sv.product_id = si.product_id
			WHERE si.id = $1
			ON CONFLICT (sale_item_id) DO NOTHING`,
			l.ItemID, j.ServiceID, j.Size, j.Color, j.Double, j.Sheets, strings.TrimSpace(j.Material)); err != nil {
			return err
		}
	}
	return nil
}
