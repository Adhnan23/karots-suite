package aicatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// JobApply is the ticked rows. Asked and Suggested say what to record for the rest:
// asked with no suggestion = no answer; suggested but left unticked = skipped.
type JobApply struct {
	Rows      []JobRow `json:"rows"`
	Asked     []int64  `json:"asked"`
	Suggested []int64  `json:"suggested"`
	State     string   `json:"state"` // from the preview: the record's key
}

// JobSkip is a ticked row that couldn't be applied, and why.
type JobSkip struct {
	ProductID int64  `json:"productId"`
	Label     string `json:"label"`
	Why       string `json:"why"`
}

// JobApplied answers an apply.
type JobApplied struct {
	Applied int       `json:"applied"`
	Skipped []JobSkip `json:"skipped"`
	Summary string    `json:"summary"`
}

func num(n int) string { return strconv.Itoa(n) }

func (h handlers) jobApply(c echo.Context) error {
	j := h.p.job(c.Param("job"))
	if j == nil {
		return apperr.NotFound("job")
	}
	var in JobApply
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	out, err := h.p.applyJob(c.Request().Context(), j, in, middleware.CurrentUserID(c))
	if err != nil {
		return apperr.Internal("apply the suggestions", err)
	}
	return c.JSON(http.StatusOK, out)
}

// applyJob gives every row its own transaction: one row that no longer fits is
// skipped and reported, and the rest still apply.
func (p *Plugin) applyJob(ctx context.Context, j *Job, in JobApply, userID int64) (JobApplied, error) {
	out := JobApplied{Skipped: []JobSkip{}}
	var undo []UndoOp
	done := map[int64]bool{}
	for _, r := range in.Rows {
		var ops []UndoOp
		err := appdb.WithTx(ctx, p.store.db, func(tx *sqlx.Tx) error {
			var active bool
			if err := tx.GetContext(ctx, &active, `SELECT COALESCE((SELECT is_active FROM products WHERE id = $1), false)`, r.ProductID); err != nil {
				return err
			}
			if !active {
				return errStale
			}
			var err error
			ops, err = j.Apply(ctx, p, tx, r)
			return err
		})
		if err != nil {
			why := skipReason(err)
			out.Skipped = append(out.Skipped, JobSkip{ProductID: r.ProductID, Label: r.Label, Why: why})
			continue
		}
		for i := range ops {
			ops[i].Kind, ops[i].Job = "job", j.Key
		}
		undo = append(undo, ops...)
		done[r.ProductID] = true
		out.Applied++
	}
	out.Summary = j.Label + ": " + num(out.Applied) + " applied"
	if len(out.Skipped) > 0 {
		out.Summary += ", " + num(len(out.Skipped)) + " no longer fit"
	}
	suggested := map[int64]bool{}
	for _, id := range in.Suggested {
		suggested[id] = true
	}
	record := j.Key
	if strings.HasPrefix(in.State, j.Key+":") {
		record = in.State
	}
	err := appdb.WithTx(ctx, p.store.db, func(tx *sqlx.Tx) error {
		for _, id := range in.Asked {
			state := "no_answer"
			switch {
			case done[id]:
				state = "applied"
			case suggested[id]:
				state = "skipped"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO aicatalog_job_items (job, product_id, state)
				SELECT $1, id, $3 FROM products WHERE id = $2
				ON CONFLICT (job, product_id) DO UPDATE SET state = $3, updated_at = now()`, record, id, state); err != nil {
				return err
			}
		}
		if len(undo) == 0 {
			return nil
		}
		raw, err := json.Marshal(undo)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO aicatalog_optimize_runs (applied_by, summary, undo) VALUES (NULLIF($1, 0), $2, $3)`, userID, out.Summary, raw)
		return err
	})
	return out, err
}

// undoJobOp is revertOp's case for a job's row: the job puts it back if nobody has
// changed it since, and the product returns to the job's list either way.
func (p *Plugin) undoJobOp(ctx context.Context, tx *sqlx.Tx, raw map[string]any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var op UndoOp
	if err := json.Unmarshal(b, &op); err != nil {
		return err
	}
	j := p.job(op.Job)
	if j == nil || j.Undo == nil {
		return nil // the plugin isn't in this build any more
	}
	if err := j.Undo(ctx, p, tx, op); err != nil && !errors.Is(err, errStale) {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM aicatalog_job_items WHERE (job = $1 OR job LIKE $1 || ':%') AND product_id = ANY($2)`, op.Job, pq.Array([]int64{op.ProductID}))
	return err
}

// staleIfNone turns "the row wasn't as we left it" into errStale, which undo ignores.
func staleIfNone(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errStale
	}
	return nil
}

// skipReason is what a person is told about a row that didn't apply: a plugin's own
// words, never a database's.
func skipReason(err error) string {
	if e, ok := apperr.As(err); ok {
		return e.Message
	}
	var pe *pq.Error
	if errors.As(err, &pe) {
		return "it couldn't be saved"
	}
	return err.Error()
}
