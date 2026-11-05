package aicatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// fakeJob renames a product from a reply of {"items":[{"id":1,"name":"x"}]}.
func fakeJob() Job {
	return Job{Key: "fake", Label: "Fake", Prompt: func(_ context.Context, _ *Plugin, b Batch) (string, string, error) {
		var ids []string
		for _, x := range b.Products {
			ids = append(ids, fmt.Sprint(x.ID))
		}
		return "SYSTEM", "IDS " + strings.Join(ids, ","), nil
	}, Parse: func(_ context.Context, _ *Plugin, _ Batch, reply string) ([]JobRow, error) {
		var r struct {
			Items []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		}
		if err := decodeReply(reply, &r); err != nil {
			return nil, err
		}
		rows := []JobRow{}
		for _, it := range r.Items {
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "name", Key: "name", New: it.Name, Ticked: true})
		}
		return rows, nil
	}, Apply: func(ctx context.Context, _ *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error) {
		var old string
		if err := tx.GetContext(ctx, &old, `SELECT name FROM products WHERE id = $1`, r.ProductID); err != nil {
			return nil, err
		}
		if r.Old != "" && old != r.Old {
			return nil, errStale
		}
		_, err := tx.ExecContext(ctx, `UPDATE products SET name = $2 WHERE id = $1`, r.ProductID, r.New)
		return []UndoOp{{ProductID: r.ProductID, Key: "name", Applied: r.New, Previous: old}}, err
	}, Undo: func(ctx context.Context, _ *Plugin, tx *sqlx.Tx, op UndoOp) error {
		res, err := tx.ExecContext(ctx, `UPDATE products SET name = $2 WHERE id = $1 AND name = $3`, op.ProductID, op.Previous, op.Applied)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errStale
		}
		return nil
	}}
}

func jobEnv(t *testing.T, name string, n int) (*Plugin, func(string, string, string) JobPreview, []int64, *sqlx.DB) {
	t.Helper()
	db := freshDB(t, name)
	p, do := testAPI(t, db)
	p.jobs = []Job{fakeJob()}
	c := cat(t, db, "Goods", nil)
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = prod(t, db, fmt.Sprintf("P%d", i), c)
	}
	next := func(job, body, reply string) JobPreview {
		t.Helper()
		var in map[string]any
		if body == "" {
			body = "{}"
		}
		if err := json.Unmarshal([]byte(body), &in); err != nil {
			t.Fatal(err)
		}
		in["reply"] = reply
		raw, _ := json.Marshal(in)
		out := decode[AskReply[JobPreview]](t, do("POST", "/jobs/"+job+"/next", string(raw)))
		if out.Result == nil {
			t.Fatalf("no result: %+v", out)
		}
		return *out.Result
	}
	return p, next, ids, db
}

func reply(items ...string) string { return `{"items":[` + strings.Join(items, ",") + `]}` }

func TestNextGivesPromptThenPreview(t *testing.T) {
	db := freshDB(t, "aicatalog_job_prompt")
	p, do := testAPI(t, db)
	p.jobs = []Job{fakeJob()}
	c := cat(t, db, "Goods", nil)
	a := prod(t, db, "A", c)
	mustExec(t, db, nil, `UPDATE aicatalog_settings SET custom_prompt = 'We sell books.'`)
	out := decode[AskReply[JobPreview]](t, do("POST", "/jobs/fake/next", `{}`))
	if out.Result != nil || !strings.Contains(out.Prompt, "SYSTEM") || !strings.Contains(out.Prompt, "We sell books.") || !strings.Contains(out.Prompt, fmt.Sprint(a)) {
		t.Fatalf("%+v", out)
	}
	bad := decode[AskReply[JobPreview]](t, do("POST", "/jobs/fake/next", `{"reply":"no json here"}`))
	if bad.Error == "" || bad.Prompt == "" {
		t.Fatalf("a bad paste keeps the prompt and says why: %+v", bad)
	}
}

func TestNextBatchSizeAndCategory(t *testing.T) {
	_, next, ids, db := jobEnv(t, "aicatalog_job_size", 4)
	if got := next("fake", `{"size":1}`, reply()); len(got.Asked) != 1 || got.Remaining != 3 || got.Asked[0] != ids[0] {
		t.Fatalf("%+v", got)
	}
	if got := next("fake", `{"size":9999}`, reply()); len(got.Asked) != 4 {
		t.Fatalf("%+v", got)
	}
	parent := cat(t, db, "Medicine", nil)
	sub := cat(t, db, "Tablets", parent)
	in := prod(t, db, "Pill", sub)
	got := next("fake", fmt.Sprintf(`{"categoryId":%d}`, parent), reply())
	if !slices.Equal(got.Asked, []int64{in}) {
		t.Fatalf("sub-categories count: %+v", got)
	}
	mustExec(t, db, nil, `UPDATE products SET is_service = true WHERE id = $1`, ids[3])
	mustExec(t, db, nil, `UPDATE products SET is_active = false WHERE id = $1`, ids[2])
	if got := next("fake", "", reply()); slices.Contains(got.Asked, ids[3]) || slices.Contains(got.Asked, ids[2]) {
		t.Fatalf("services and disabled products aren't offered: %+v", got)
	}
}

func TestParseDropsForeignIDs(t *testing.T) {
	_, next, ids, _ := jobEnv(t, "aicatalog_job_foreign", 2)
	got := next("fake", `{"size":1}`, reply(fmt.Sprintf(`{"id":%d,"name":"ok"}`, ids[0]), fmt.Sprintf(`{"id":%d,"name":"not asked"}`, ids[1]), `{"id":999999,"name":"no such"}`))
	if len(got.Rows) != 1 || got.Rows[0].ProductID != ids[0] {
		t.Fatalf("%+v", got.Rows)
	}
}

func TestApplyMarksAndNextSkips(t *testing.T) {
	p, next, ids, db := jobEnv(t, "aicatalog_job_states", 4)
	ctx := context.Background()
	j := p.job("fake")
	res, err := p.applyJob(ctx, j, JobApply{Rows: []JobRow{{ProductID: ids[0], Kind: "name", Key: "name", New: "Done"}},
		Asked: ids[:3], Suggested: ids[:2]}, 0)
	if err != nil || res.Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	state := func(id int64) string {
		var s string
		_ = db.Get(&s, `SELECT state FROM aicatalog_job_items WHERE job = 'fake' AND product_id = $1`, id)
		return s
	}
	if state(ids[0]) != "applied" || state(ids[1]) != "skipped" || state(ids[2]) != "no_answer" || state(ids[3]) != "" {
		t.Fatalf("%s %s %s %s", state(ids[0]), state(ids[1]), state(ids[2]), state(ids[3]))
	}
	if got := next("fake", "", reply()); !slices.Equal(got.Asked, []int64{ids[3]}) {
		t.Fatalf("answered products come back: %+v", got)
	}
	if got := next("fake", `{"filter":"skipped"}`, reply()); !slices.Equal(got.Asked, []int64{ids[1], ids[2]}) {
		t.Fatalf("filter: %+v", got)
	}
	if got := next("fake", fmt.Sprintf(`{"ids":[%d]}`, ids[0]), reply()); !slices.Equal(got.Asked, []int64{ids[0]}) {
		t.Fatalf("redo: %+v", got)
	}
}

func TestApplySkipsChanged(t *testing.T) {
	p, _, ids, db := jobEnv(t, "aicatalog_job_stale", 3)
	mustExec(t, db, nil, `UPDATE products SET is_active = false WHERE id = $1`, ids[0])
	mustExec(t, db, nil, `UPDATE products SET name = 'Hand edited' WHERE id = $1`, ids[1])
	res, err := p.applyJob(context.Background(), p.job("fake"), JobApply{Rows: []JobRow{
		{ProductID: ids[0], New: "x", Label: "disabled"},
		{ProductID: ids[1], Old: "P1", New: "y", Label: "edited"},
		{ProductID: ids[2], Old: "P2", New: "z", Label: "fine"},
		{ProductID: 99999999, New: "gone", Label: "gone"},
	}, Asked: ids}, 0)
	if err != nil || res.Applied != 1 || len(res.Skipped) != 3 {
		t.Fatalf("%+v %v", res, err)
	}
	var name string
	mustExec2(t, db, &name, `SELECT name FROM products WHERE id = $1`, ids[1])
	if name != "Hand edited" {
		t.Fatalf("overwrote a newer edit: %q", name)
	}
	mustExec2(t, db, &name, `SELECT name FROM products WHERE id = $1`, ids[2])
	if name != "z" {
		t.Fatalf("the good row: %q", name)
	}
}

func mustExec2(t *testing.T, db *sqlx.DB, dest *string, q string, args ...any) {
	t.Helper()
	if err := db.Get(dest, q, args...); err != nil {
		t.Fatal(err)
	}
}

func TestUndoLeavesNewerEdit(t *testing.T) {
	p, _, ids, db := jobEnv(t, "aicatalog_job_undo", 2)
	ctx := context.Background()
	if _, err := p.applyJob(ctx, p.job("fake"), JobApply{Rows: []JobRow{{ProductID: ids[0], New: "A2"}, {ProductID: ids[1], New: "B2"}}, Asked: ids, Suggested: ids}, 0); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, nil, `UPDATE products SET name = 'Hand edited' WHERE id = $1`, ids[1])
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	var a, b string
	mustExec2(t, db, &a, `SELECT name FROM products WHERE id = $1`, ids[0])
	mustExec2(t, db, &b, `SELECT name FROM products WHERE id = $1`, ids[1])
	if a != "P0" || b != "Hand edited" {
		t.Fatalf("%q %q", a, b)
	}
	if n := count(t, db, `SELECT count(*) FROM aicatalog_job_items WHERE job = 'fake'`); n != 0 {
		t.Fatalf("undone products go back on the list: %d", n)
	}
}

func TestUnavailableJob(t *testing.T) {
	db := freshDB(t, "aicatalog_job_unavailable")
	p, do := testAPI(t, db)
	off := fakeJob()
	off.Available = func(*Plugin) bool { return false }
	p.jobs = []Job{off}
	if list := decode[[]JobInfo](t, do("GET", "/jobs", "")); len(list) != 0 {
		t.Fatalf("%+v", list)
	}
	if rec := do("POST", "/jobs/fake/next", `{}`); rec.Code != 404 {
		t.Fatalf("next: %d", rec.Code)
	}
	if rec := do("POST", "/jobs/fake/apply", `{}`); rec.Code != 404 {
		t.Fatalf("apply: %d", rec.Code)
	}
}

func TestSkipReasonHidesTheDatabase(t *testing.T) {
	if got := skipReason(&pq.Error{Code: "22003", Message: "numeric field overflow"}); strings.Contains(got, "overflow") || strings.Contains(got, "pq") {
		t.Fatal(got)
	}
	if got := skipReason(errors.New("Weight: not a number")); got != "Weight: not a number" {
		t.Fatal(got)
	}
	if got := skipReason(apperr.Validation("pick one")); got != "pick one" {
		t.Fatal(got)
	}
}

func TestLeaveAndContinue(t *testing.T) {
	p, next, ids, _ := jobEnv(t, "aicatalog_job_continue", 3)
	if _, err := p.applyJob(context.Background(), p.job("fake"), JobApply{Rows: []JobRow{{ProductID: ids[0], New: "Done"}}, Asked: ids[:1], Suggested: ids[:1]}, 0); err != nil {
		t.Fatal(err)
	}
	// Coming back later: only what's left, and asking without applying changes nothing.
	for i := 0; i < 2; i++ {
		if got := next("fake", `{"size":1}`, reply()); !slices.Equal(got.Asked, []int64{ids[1]}) {
			t.Fatalf("%+v", got)
		}
	}
}

func TestForceRedoHasItsOwnRecord(t *testing.T) {
	p, next, ids, db := jobEnv(t, "aicatalog_job_force", 2)
	ctx, j := context.Background(), func() *Job { return p.job("fake") }
	if _, err := p.applyJob(ctx, j(), JobApply{Asked: ids, Suggested: ids, Rows: []JobRow{{ProductID: ids[0], New: "A"}, {ProductID: ids[1], New: "B"}}}, 0); err != nil {
		t.Fatal(err)
	}
	if got := next("fake", "", reply()); len(got.Asked) != 0 {
		t.Fatalf("all done: %+v", got)
	}
	got := next("fake", `{"force":true,"size":1}`, reply())
	if !slices.Equal(got.Asked, []int64{ids[0]}) || got.State != "fake:redo" || got.Remaining != 1 {
		t.Fatalf("redo starts from the first: %+v", got)
	}
	if _, err := p.applyJob(ctx, j(), JobApply{State: got.State, Asked: ids[:1], Suggested: ids[:1]}, 0); err != nil {
		t.Fatal(err)
	}
	if got := next("fake", `{"force":true,"size":1}`, reply()); !slices.Equal(got.Asked, []int64{ids[1]}) {
		t.Fatalf("a redo continues too: %+v", got)
	}
	if got := next("fake", "", reply()); len(got.Asked) != 0 {
		t.Fatalf("the normal list isn't reopened: %+v", got)
	}
	if _, err := db.Exec(`DELETE FROM aicatalog_job_items WHERE job LIKE $1 || ':%redo'`, "fake"); err != nil {
		t.Fatal(err)
	}
	if got := next("fake", `{"force":true,"size":1}`, reply()); !slices.Equal(got.Asked, []int64{ids[0]}) {
		t.Fatalf("start over: %+v", got)
	}
}
