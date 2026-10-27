package gen

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jmoiron/sqlx"
)

// The time machine. Every service writes "now", so history is generated as if
// it all happened today and then moved: before each operation the generator
// marks the moment it *should* have happened; when the batch ends, every row
// written since the batch began has its timestamps moved by its operation's
// offset, and its dates by the same number of days. It covers every table —
// core and plugin — without a list to keep up to date.
//
// Rows are found by xmin (the transaction that last wrote them) and a timestamp
// at or after the batch's start, so history already moved is never moved twice.

type anchor struct{ wall, target time.Time }

type timeMachine struct {
	db       *sqlx.DB
	startXID int64
	start    time.Time
	anchors  []anchor
	tsCols   []col
	dateCols []col
}

type col struct{ table, column string }

func newTimeMachine(ctx context.Context, db *sqlx.DB) (*timeMachine, error) {
	tm := &timeMachine{db: db}
	var rows []struct {
		Table  string `db:"table_name"`
		Column string `db:"column_name"`
		Type   string `db:"data_type"`
	}
	err := db.SelectContext(ctx, &rows, `
		SELECT c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t USING (table_schema, table_name)
		WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		  AND c.data_type IN ('timestamp with time zone', 'timestamp without time zone', 'date')
		  AND c.table_name NOT LIKE 'goose_db_version%' AND c.table_name <> 'demo_anchors'
		ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Type == "date" {
			tm.dateCols = append(tm.dateCols, col{r.Table, r.Column})
		} else {
			tm.tsCols = append(tm.tsCols, col{r.Table, r.Column})
		}
	}
	_, err = db.ExecContext(ctx, `CREATE UNLOGGED TABLE IF NOT EXISTS demo_anchors (wall timestamptz PRIMARY KEY, target timestamptz NOT NULL)`)
	return tm, err
}

// begin starts a batch.
func (tm *timeMachine) begin(ctx context.Context) error {
	tm.anchors = tm.anchors[:0]
	tm.start = time.Now()
	return tm.db.GetContext(ctx, &tm.startXID, `SELECT pg_current_xact_id()::text::bigint`)
}

// at says the operation that follows happened at target.
func (tm *timeMachine) at(target time.Time) {
	w := time.Now()
	if n := len(tm.anchors); n > 0 && !w.After(tm.anchors[n-1].wall) {
		w = tm.anchors[n-1].wall.Add(time.Microsecond) // keep walls unique and rising
	}
	tm.anchors = append(tm.anchors, anchor{w, target})
}

// shift moves the batch into the past. Dates go first, found through their
// row's own created_at while it still holds the real time (no created_at: the
// batch's first day).
func (tm *timeMachine) shift(ctx context.Context) error {
	if len(tm.anchors) == 0 {
		return nil
	}
	tx, err := tm.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `TRUNCATE demo_anchors`); err != nil {
		return err
	}
	// Writes before the first mark belong to the first operation.
	first := anchor{tm.start.Add(-time.Second), tm.anchors[0].target.Add(-time.Second)}
	for _, a := range append([]anchor{first}, tm.anchors...) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO demo_anchors VALUES ($1, $2)`, a.wall, a.target); err != nil {
			return err
		}
	}
	lastWall := time.Now().Add(time.Second)
	startDay := tm.start.Format("2006-01-02")
	firstDays := int(dayOf(tm.anchors[0].target).Sub(dayOf(tm.start)).Hours() / 24)
	hasCreated := map[string]bool{}
	for _, c := range tm.tsCols {
		if c.column == "created_at" {
			hasCreated[c.table] = true
		}
	}
	for _, c := range tm.dateCols {
		by := fmt.Sprintf("%d", firstDays)
		if hasCreated[c.table] {
			by = fmt.Sprintf(`COALESCE((SELECT (a.target AT TIME ZONE 'UTC')::date - (a.wall AT TIME ZONE 'UTC')::date
				FROM demo_anchors a WHERE a.wall <= t.created_at ORDER BY a.wall DESC LIMIT 1), %d)`, firstDays)
		}
		q := fmt.Sprintf(`UPDATE %q AS t SET %q = t.%q + %s
			WHERE t.xmin::text::bigint >= $1 AND t.%q IS NOT NULL AND t.%q >= $2::date - 1`,
			c.table, c.column, c.column, by, c.column, c.column)
		if hasCreated[c.table] {
			q += ` AND t.created_at >= $3`
		}
		args := []any{tm.startXID, startDay}
		if hasCreated[c.table] {
			args = append(args, tm.start)
		}
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("move %s.%s: %w", c.table, c.column, err)
		}
	}
	for _, c := range tm.tsCols {
		q := fmt.Sprintf(`UPDATE %q AS t SET %q = t.%q + (SELECT a.target - a.wall
				FROM demo_anchors a WHERE a.wall <= t.%q ORDER BY a.wall DESC LIMIT 1)
			WHERE t.xmin::text::bigint >= $1 AND t.%q >= $2 AND t.%q <= $3`,
			c.table, c.column, c.column, c.column, c.column, c.column)
		if _, err := tx.ExecContext(ctx, q, tm.startXID, tm.start.Add(-time.Second), lastWall); err != nil {
			return fmt.Errorf("move %s.%s: %w", c.table, c.column, err)
		}
	}
	return tx.Commit()
}

// daysToMove is how many whole days a past whose last trade was at last moves so
// it ends the day before now — counted in local calendar days (Sri Lanka is
// UTC+5:30: its midnight is the previous day in UTC).
func daysToMove(now, last time.Time) int {
	gap := localDay(now).Sub(localDay(last.In(now.Location())))
	return int(math.Round(gap.Hours()/24)) - 1
}

func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// moveAll moves a whole past forward by whole days so its last day of trade is
// the day before today (a restored backup made on an earlier day). Every
// timestamp and date in every table moves the same way, so nothing drifts apart.
func (tm *timeMachine) moveAll(ctx context.Context, today time.Time) error {
	var last time.Time
	if err := tm.db.GetContext(ctx, &last, `SELECT COALESCE(max(created_at), now()) FROM sales`); err != nil {
		return err
	}
	days := daysToMove(today, last)
	if days <= 0 {
		return nil
	}
	tx, err := tm.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range tm.tsCols {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %q SET %q = %q + make_interval(days => $1) WHERE %q IS NOT NULL`,
			c.table, c.column, c.column, c.column), days); err != nil {
			return fmt.Errorf("move %s.%s: %w", c.table, c.column, err)
		}
	}
	for _, c := range tm.dateCols {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %q SET %q = %q + $1::int WHERE %q IS NOT NULL`,
			c.table, c.column, c.column, c.column), days); err != nil {
			return fmt.Errorf("move %s.%s: %w", c.table, c.column, err)
		}
	}
	return tx.Commit()
}
