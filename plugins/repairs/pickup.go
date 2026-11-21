package repairs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// recordPickup marks each job whose lines are on the sale collected, inside the
// sale's tx (plugin.SaleLineHook): the job and its sale can't get out of step.
// It never refuses — a job already collected or cancelled is left as it is, and
// the sale stands whoever paid (the device left the shop with them).
func recordPickup(ctx context.Context, tx *sqlx.Tx, ev plugin.SaleLines) error {
	var order []int64
	lines := map[int64][]int64{}
	for _, l := range ev.Lines {
		var tag struct {
			Job int64 `json:"job"`
		}
		if json.Unmarshal(l.Data, &tag) != nil || tag.Job <= 0 {
			continue
		}
		if _, ok := lines[tag.Job]; !ok {
			order = append(order, tag.Job)
		}
		lines[tag.Job] = append(lines[tag.Job], l.ItemID)
	}
	for _, job := range order {
		if err := collectTx(ctx, tx, job, ev.SaleID, ev.ActorID, lines[job], time.Now()); err != nil {
			return err
		}
	}
	return nil
}

// collectTx links the pickup sale and starts the warranty: by days = the job's
// own; by part = the latest part's; a free re-repair runs on the original's —
// and keeps which of the sale's lines were the job's (the report's, REP-8).
func collectTx(ctx context.Context, tx *sqlx.Tx, jobID, saleID, actorID int64, items []int64, now time.Time) error {
	var j struct {
		TicketNo     string `db:"ticket_no"`
		Status       string `db:"status"`
		WarrantyDays int    `db:"warranty_days"`
		ReworkOf     *int64 `db:"rework_of"`
		Mode         string `db:"warranty_mode"`
		PartDays     int    `db:"part_days"`
	}
	err := tx.GetContext(ctx, &j, `
		SELECT j.ticket_no, j.status, j.warranty_days, j.rework_of,
		       COALESCE((SELECT warranty_mode FROM repair_config WHERE id = 1), 'days') AS warranty_mode,
		       COALESCE((SELECT MAX(warranty_days) FROM repair_parts WHERE job_id = j.id), 0) AS part_days
		FROM repair_jobs j WHERE j.id = $1 FOR UPDATE OF j`, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // a line from a job that's gone: the sale still stands
	}
	if err != nil || j.Status == "collected" || j.Status == "cancelled" {
		return err
	}
	var until *time.Time
	if j.ReworkOf == nil {
		until = jobWarrantyUntil(j.Mode, &Detail{Job: Job{WarrantyDays: j.WarrantyDays}, Parts: []Part{{WarrantyDays: j.PartDays}}}, now)
	}
	if ok, err := (&Store{}).MarkCollected(ctx, tx, jobID, saleID, until); err != nil || !ok {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO repair_pickup_lines (job_id, sale_item_id) SELECT $1, unnest($2::bigint[])`,
		jobID, pq.Array(items)); err != nil {
		return err
	}
	return audit.NewRepository(tx).Insert(ctx, actorID, audit.ActionUpdate, "repair", strconv.FormatInt(jobID, 10),
		"repair collected: "+j.TicketNo+" (sale "+strconv.FormatInt(saleID, 10)+")")
}
