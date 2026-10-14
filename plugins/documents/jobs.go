package documents

import (
	"context"
	"database/sql"
	"errors"
	"karots-suite/internal/features/reports"
	"time"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"

	"github.com/shopspring/decimal"
)

// SoldJob is one Print & Copy line of a sale, as the till reports it after the
// sale. Money is NOT taken from it: the ledger copies the sale line's own qty,
// price and cost, so a wrong or replayed post can't invent revenue.
type SoldJob struct {
	ServiceID   int64
	Description string
	Size        *string
	Color       *bool
	Double      *bool
	Sheets      *decimal.Decimal
}

// RecordSold writes one doc_job per job, each linked to a line of the sale with
// the service's product and the job's description that no job holds yet.
// A job matching no line is refused (nothing is written).
func (s *Store) RecordSold(ctx context.Context, saleID int64, jobs []SoldJob) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, j := range jobs {
		var itemID int64
		err := tx.GetContext(ctx, &itemID, `
			SELECT si.id FROM sale_items si JOIN doc_service sv ON sv.product_id = si.product_id
			WHERE si.sale_id = $1 AND sv.id = $2 AND COALESCE(si.description, '') = $3
			  AND NOT EXISTS (SELECT 1 FROM doc_job j WHERE j.sale_item_id = si.id)
			ORDER BY si.id LIMIT 1 FOR UPDATE OF si`, saleID, j.ServiceID, j.Description)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.Validation("a job doesn't match a line of that sale")
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO doc_job (sale_id, sale_item_id, service_id, description, qty, unit_price, line_total,
			                     consumable_cost, kind, size, color, double_side, sheets)
			SELECT si.sale_id, si.id, $2, $3, si.quantity, si.unit_price, si.subtotal,
			       ROUND(si.cost_price * si.quantity, 2), 'sale', $4, $5, $6, $7
			FROM sale_items si WHERE si.id = $1`, itemID, j.ServiceID, j.Description, j.Size, j.Color, j.Double, j.Sheets); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecordOwnUse logs a shop-use job (no sale, no price).
func (s *Store) RecordOwnUse(ctx context.Context, tx sqlx.ExecerContext, sv *Service, desc string, o JobOptions, cost decimal.Decimal) error {
	size := &o.Size
	if o.Size == "" {
		size = nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO doc_job (service_id, description, qty, unit_price, line_total, consumable_cost, kind,
		                     size, color, double_side, sheets)
		VALUES ($1, $2, $3, 0, 0, $4, 'own_use', $5, $6, $7, $8)`,
		sv.ID, desc, o.Copies, cost, size, o.Color, o.Double, Sheets(o))
	return err
}

// ReportRow is one service × options over a period. Linked = a suite job (its
// sale line's returns are taken off); an unlinked sale job is a karots-pos one.
type ReportRow struct {
	Name    string          `db:"name"`
	Size    *string         `db:"size"`
	Color   *bool           `db:"color"`
	Double  *bool           `db:"double_side"`
	Linked  bool            `db:"linked"`
	Jobs    int             `db:"jobs"`
	Copies  decimal.Decimal `db:"copies"`
	Sheets  decimal.Decimal `db:"sheets"`
	Revenue decimal.Decimal `db:"revenue"`
}

type Report struct {
	Rows    []ReportRow
	Jobs    int
	Copies  decimal.Decimal
	Sheets  decimal.Decimal
	Revenue decimal.Decimal
	OwnUse  OwnUse
}

// OwnUse is the shop's own jobs: paper used, nothing sold.
type OwnUse struct {
	Jobs   int             `db:"jobs"`
	Sheets decimal.Decimal `db:"sheets"`
	Cost   decimal.Decimal `db:"cost"`
}

// Report sums jobs made in [from, to). Copies and revenue are what was kept
// (returned copies off, a bill discount shared out as core counts a line; a
// karots-pos reversal leaves its job out); sheets are
// what was printed, kept or not — that paper is gone either way.
func (s *Store) Report(ctx context.Context, from, to time.Time) (Report, error) {
	r := Report{Rows: []ReportRow{}}
	err := s.db.SelectContext(ctx, &r.Rows, `
		SELECT COALESCE(p.name, '(deleted service)') AS name, j.size, j.color, j.double_side,
		       j.sale_item_id IS NOT NULL AS linked, COUNT(*) AS jobs,
		       COALESCE(SUM(CASE WHEN si.id IS NULL THEN j.qty ELSE si.quantity - si.returned_qty END), 0) AS copies,
		       COALESCE(SUM(j.sheets), 0) AS sheets,
		       COALESCE(SUM(CASE WHEN si.id IS NULL THEN j.line_total
		                         ELSE `+reports.KeptRevenueSQL+` END), 0) AS revenue
		FROM doc_job j
		LEFT JOIN doc_service sv ON sv.id = j.service_id
		LEFT JOIN products p ON p.id = sv.product_id
		LEFT JOIN sale_items si ON si.id = j.sale_item_id
		LEFT JOIN sales s ON s.id = j.sale_id
		WHERE j.kind = 'sale' AND j.reversed_at IS NULL AND (s.status IS NULL OR s.status <> 'void')
		  AND j.created_at >= $1 AND j.created_at < $2
		GROUP BY 1, 2, 3, 4, 5
		ORDER BY revenue DESC, name`, from, to)
	if err != nil {
		return r, err
	}
	r.Copies, r.Sheets, r.Revenue = decimal.Zero, decimal.Zero, decimal.Zero
	for i, x := range r.Rows {
		r.Rows[i].Revenue = x.Revenue.Round(2)
		r.Jobs += x.Jobs
		r.Copies = r.Copies.Add(x.Copies)
		r.Sheets = r.Sheets.Add(x.Sheets)
		r.Revenue = r.Revenue.Add(r.Rows[i].Revenue)
	}
	err = s.db.GetContext(ctx, &r.OwnUse, `
		SELECT COUNT(*) AS jobs, COALESCE(SUM(sheets), 0) AS sheets, COALESCE(SUM(consumable_cost), 0) AS cost
		FROM doc_job WHERE kind = 'own_use' AND created_at >= $1 AND created_at < $2`, from, to)
	return r, err
}
