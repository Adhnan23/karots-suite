package combos

import (
	"context"

	"github.com/shopspring/decimal"
)

// ReportRow is one combo over a period. Name is the name it sold under (a
// renamed or deleted combo keeps its history).
type ReportRow struct {
	ComboID  *int64          `db:"combo_id"`
	Name     string          `db:"name"`
	Sets     decimal.Decimal `db:"sets"`
	Net      decimal.Decimal `db:"net"`
	Discount decimal.Decimal `db:"discount"`
	Bills    int             `db:"bills"`
}

type Report struct {
	Rows   []ReportRow
	Totals ReportRow
}

// Report sums combo_sales per combo for sales dated from..to (inclusive,
// YYYY-MM-DD, the sale's own date), leaving voided sales out.
func (s *Store) Report(ctx context.Context, from, to string) (Report, error) {
	var r Report
	err := s.db.SelectContext(ctx, &r.Rows, `
		SELECT cs.combo_id, MAX(cs.name) AS name, SUM(cs.sets) AS sets, SUM(cs.net) AS net,
		       SUM(cs.discount) AS discount, COUNT(DISTINCT cs.sale_id) AS bills
		FROM combo_sales cs JOIN sales s ON s.id = cs.sale_id
		WHERE s.status <> 'void' AND s.created_at::date BETWEEN $1::date AND $2::date
		GROUP BY cs.combo_id, CASE WHEN cs.combo_id IS NULL THEN cs.name END
		ORDER BY sets DESC, name`, from, to)
	if err != nil {
		return r, err
	}
	for _, x := range r.Rows {
		r.Totals.Sets = r.Totals.Sets.Add(x.Sets)
		r.Totals.Net = r.Totals.Net.Add(x.Net)
		r.Totals.Discount = r.Totals.Discount.Add(x.Discount)
		r.Totals.Bills += x.Bills
	}
	return r, nil
}
