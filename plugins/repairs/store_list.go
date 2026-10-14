package repairs

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ListFilter: Show is open (received / in progress / ready) | ready | collected |
// cancelled | all; Q matches ticket, customer, phone, device (an exact ticket
// first, any status); From/To bound the drop-off date.
type ListFilter struct {
	Show     string
	Q        string
	From, To *time.Time
	Limit    int
}

var showStatuses = map[string]string{
	"open": `'received','in_progress','ready'`, "ready": `'ready'`, "collected": `'collected'`, "cancelled": `'cancelled'`,
}

func (s *Store) List(ctx context.Context, f ListFilter) ([]Job, error) {
	q := `SELECT * FROM repair_jobs WHERE true`
	var args []any
	arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
	if term := strings.TrimSpace(f.Q); term != "" {
		p := arg("%" + term + "%")
		q += ` AND (ticket_no ILIKE ` + p + ` OR ticket_code ILIKE ` + p + ` OR customer_name ILIKE ` + p +
			` OR customer_phone ILIKE ` + p + ` OR device_model ILIKE ` + p + ` OR repair_type ILIKE ` + p + `)`
	} else if in, ok := showStatuses[f.Show]; ok {
		q += ` AND status IN (` + in + `)`
	}
	if f.From != nil {
		q += ` AND created_at >= ` + arg(*f.From)
	}
	if f.To != nil {
		q += ` AND created_at < ` + arg(*f.To)
	}
	order := ` ORDER BY urgent DESC, (promised_date IS NULL), promised_date, created_at`
	switch {
	case strings.TrimSpace(f.Q) != "":
		e := arg(strings.ToUpper(strings.TrimSpace(f.Q)))
		order = ` ORDER BY (upper(ticket_code) = ` + e + ` OR upper(ticket_no) = ` + e + `) DESC, created_at DESC`
	case f.Show == "collected":
		order = ` ORDER BY collected_at DESC`
	case f.Show == "cancelled" || f.Show == "all":
		order = ` ORDER BY created_at DESC`
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q += order + ` LIMIT ` + arg(limit)
	rows := []Job{}
	err := s.db.SelectContext(ctx, &rows, q, args...)
	return rows, err
}

// Suggestions are what was typed before, most used first.
type Suggestions struct {
	Types     []string `json:"types"`
	Models    []string `json:"models"`
	Repairers []string `json:"repairers"`
	Labour    []string `json:"labour"`
}

func (s *Store) Suggestions(ctx context.Context) (Suggestions, error) {
	out := Suggestions{Types: []string{}, Models: []string{}, Repairers: []string{}, Labour: []string{}}
	for _, x := range []struct {
		dst *[]string
		q   string
	}{
		{&out.Types, `SELECT repair_type FROM repair_jobs WHERE repair_type <> '' GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 50`},
		{&out.Models, `SELECT device_model FROM repair_jobs WHERE device_model <> '' GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 100`},
		{&out.Repairers, `SELECT repaired_by FROM repair_jobs WHERE repaired_by <> '' GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 30`},
		{&out.Labour, `SELECT label FROM repair_charges WHERE label <> '' GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 30`},
	} {
		if err := s.db.SelectContext(ctx, x.dst, x.q); err != nil {
			return out, err
		}
	}
	return out, nil
}

// ReportRow is one collected job: revenue and cost are its pickup sale's lines
// of the job's parts and the labour product, net of returns (a bill discount on
// the sale is not spread onto them — ponytail: spread it if shops use it on repairs).
type ReportRow struct {
	ID          int64           `db:"id"`
	TicketNo    string          `db:"ticket_no"`
	Device      string          `db:"device_model"`
	Customer    string          `db:"customer_name"`
	CollectedAt time.Time       `db:"collected_at"`
	ReworkOf    *int64          `db:"rework_of"`
	Revenue     decimal.Decimal `db:"revenue"`
	Cost        decimal.Decimal `db:"cost"`
	Repairer    decimal.Decimal `db:"repairer"`
}

func (s *Store) Report(ctx context.Context, from, to time.Time, labour int64) ([]ReportRow, error) {
	rows := []ReportRow{}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT j.id, j.ticket_no, j.device_model, j.customer_name, j.collected_at, j.rework_of,
		       COALESCE(r.revenue, 0)::numeric(14,2) AS revenue, COALESCE(r.cost, 0)::numeric(14,2) AS cost,
		       j.repairer_paid + COALESCE((SELECT SUM(e.amount) FROM repair_payouts rp JOIN expenses e ON e.id = rp.expense_id
		                                    WHERE rp.job_id = j.id), 0) AS repairer
		FROM repair_jobs j
		LEFT JOIN LATERAL (
			SELECT SUM(CASE WHEN si.quantity > 0 THEN si.subtotal * (si.quantity - si.returned_qty) / si.quantity ELSE 0 END) AS revenue,
			       SUM(si.cost_price * (si.quantity - si.returned_qty)) AS cost
			FROM sale_items si
			WHERE si.sale_id = j.sale_id
			  AND (si.product_id = $3 OR si.product_id IN (SELECT product_id FROM repair_parts WHERE job_id = j.id))
		) r ON true
		WHERE j.status = 'collected' AND j.collected_at >= $1 AND j.collected_at < $2
		ORDER BY j.collected_at`, from, to, labour)
	return rows, err
}
