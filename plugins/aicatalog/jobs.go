package aicatalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// A job is one kind of suggestion over many products: a prompt for a batch, a reply
// read into rows, a person's ticks, apply, undo. Each product's answer is recorded,
// so a long list keeps moving: it comes back only from the filter or a redo.

const (
	jobBatchDefault = 10
	jobBatchMax     = 50
	jobSelectMax    = 100000
)

// JobProduct is a product as a job sees it.
type JobProduct struct {
	ID         int64  `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	CategoryID int64  `db:"category_id" json:"categoryId"`
	Category   string `db:"-" json:"category"` // its path, "" = none
	Specs      string `db:"specs" json:"specs"`
	Explain    string `db:"user_explanation" json:"explanation"`
	HasHint    bool   `db:"has_hint" json:"hasHint"`
}

// Hint is the chatbot's words on it, for a prompt.
func (p JobProduct) Hint() string { return strings.TrimSpace(p.Specs + " " + p.Explain) }

// Batch is what a job's prompt and parser work on.
type Batch struct {
	Products []JobProduct
	Context  []JobProduct // other products the prompt may name, but not to be placed
	Options  json.RawMessage
	Redo     bool
	Settings Settings
	State    string // the record's key, when a job's list depends on its options ("" = the job's)
}

// JobRow is one suggestion, one tick box. Old and New are what the person compares.
type JobRow struct {
	ProductID int64             `json:"productId"`
	Kind      string            `json:"kind"`
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Old       string            `json:"old"`
	New       string            `json:"new"`
	Reason    string            `json:"reason"`
	IsNew     bool              `json:"isNew"`  // not on the shop's list: look before ticking
	Change    bool              `json:"change"` // differs from a value the shop has
	Ticked    bool              `json:"ticked"`
	Needs     string            `json:"needs,omitempty"`   // what must be filled in first, "" = nothing
	Choices   []JobChoice       `json:"choices,omitempty"` // Needs is answered by picking one
	Extra     map[string]string `json:"extra,omitempty"`
}

// JobChoice is one answer to a row's Needs; it lands in Extra[JobRow.Key].
type JobChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// UndoOp is what's needed to put a row back, as long as nobody has changed it since.
type UndoOp struct {
	Kind      string            `json:"kind"` // always "job": revertOp's switch
	Job       string            `json:"job"`
	RowKind   string            `json:"rowKind"`
	ProductID int64             `json:"productId"`
	Key       string            `json:"key"`
	Applied   string            `json:"applied"`
	Previous  string            `json:"previous"`
	Extra     map[string]string `json:"extra,omitempty"`
	Also      []int64           `json:"also,omitempty"` // other products this row settled
}

// errStale says a row no longer fits what's there: it is skipped and reported.
var errStale = errors.New("changed since the suggestion")

// Job is one kind of suggestion.
type Job struct {
	Key, Label, Desc string
	Warning          string
	OneByOne         bool // no tick-all: each row is a decision of its own
	ListNoAnswer     bool // what the chatbot wouldn't answer stays in view on the job's page
	Available        func(*Plugin) bool
	Options          func(ctx context.Context, p *Plugin) (any, error)
	Select           func(ctx context.Context, p *Plugin, in JobRequest) (Batch, int, error) // nil = selectProducts
	Prompt           func(ctx context.Context, p *Plugin, b Batch) (system, user string, err error)
	Parse            func(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error)
	Apply            func(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error)
	Undo             func(ctx context.Context, p *Plugin, tx *sqlx.Tx, op UndoOp) error
}

// JobRequest asks for the next batch; Reply carries a pasted answer.
type JobRequest struct {
	Size       int             `json:"size"`
	CategoryID int64           `json:"categoryId"`
	Filter     string          `json:"filter"` // "" = not answered yet | "skipped" = skipped or no answer
	IDs        []int64         `json:"ids"`    // a redo: exactly these
	Options    json.RawMessage `json:"options"`
	Reply      string          `json:"reply"`
	Force      bool            `json:"force"` // redo everything: has its own record, so it can continue too

	exclude     []int64 // never these (a job's own rule)
	needDetails bool    // only products a prompt could judge: a digit in the name, or a hint
	byName      bool    // alphabetical, so similar names land in one batch
	wide        bool    // whatever the job's record says
	noCap       bool    // every match: the job narrows them itself
	noHint      bool    // only products with no hint yet
	moved       bool    // an answered product comes back if its category has changed since
}

// JobPreview is the rows to tick. Asked is the whole batch, so apply can record a
// product the chatbot said nothing about.
type JobPreview struct {
	Rows      []JobRow         `json:"rows"`
	Asked     []int64          `json:"asked"`
	Names     map[int64]string `json:"names"`
	Remaining int              `json:"remaining"`
	State     string           `json:"state,omitempty"`
}

// JobInfo is a job on the page's list.
type JobInfo struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Desc     string   `json:"desc"`
	Warning  string   `json:"warning"`
	OneByOne bool     `json:"oneByOne"`
	Options  any      `json:"options,omitempty"`
	NoAnswer []string `json:"noAnswer,omitempty"`
}

func (p *Plugin) job(key string) *Job {
	for i := range p.jobs {
		if j := &p.jobs[i]; j.Key == key && (j.Available == nil || j.Available(p)) {
			return j
		}
	}
	return nil
}

// recordKey is where a job's answers are kept: a forced redo keeps its own, so leaving
// halfway continues there too, and the normal list is left as it was.
func (in JobRequest) recordKey(key string) string {
	if in.Force {
		return key + ":redo"
	}
	return key
}

func (h handlers) jobRoutes(a plugin.Group) {
	a.GET("/aicatalog/jobs", h.jobList)
	a.POST("/aicatalog/jobs/:job/next", h.jobNext)
	a.POST("/aicatalog/jobs/:job/apply", h.jobApply)
	a.POST("/aicatalog/jobs/:job/redo-over", h.jobRedoOver)
}

func (h handlers) jobList(c echo.Context) error {
	ctx := c.Request().Context()
	out := []JobInfo{}
	for _, j := range h.p.jobs {
		if j.Available != nil && !j.Available(h.p) {
			continue
		}
		info := JobInfo{Key: j.Key, Label: j.Label, Desc: j.Desc, Warning: j.Warning, OneByOne: j.OneByOne}
		if j.Options != nil {
			opts, err := j.Options(ctx, h.p)
			if err != nil {
				return apperr.Internal("read the job's options", err)
			}
			info.Options = opts
		}
		if j.ListNoAnswer {
			if err := h.p.store.db.SelectContext(ctx, &info.NoAnswer, `SELECT name FROM (
				SELECT DISTINCT ON (p.id) p.name, j.state FROM aicatalog_job_items j JOIN products p ON p.id = j.product_id
				WHERE (j.job = $1 OR j.job LIKE $1 || ':%') AND p.is_active ORDER BY p.id, j.updated_at DESC, j.job DESC
				) latest WHERE state = 'no_answer' ORDER BY name LIMIT 200`, j.Key); err != nil {
				return apperr.Internal("read the job's list", err)
			}
		}
		out = append(out, info)
	}
	return c.JSON(http.StatusOK, out)
}

// selectProducts is the usual batch: active products (not services) in id
// order, optionally inside one category and its sub-categories.
func selectProducts(ctx context.Context, p *Plugin, job string, in JobRequest) ([]JobProduct, int, error) {
	where := `p.is_active AND NOT p.is_service`
	args := []any{job}
	switch {
	case len(in.IDs) > 0:
		args = append(args, pq.Array(in.IDs))
		where += ` AND p.id = ANY($2)`
	case in.Filter == "skipped":
		where += ` AND j.state IN ('skipped', 'no_answer')`
	case in.wide:
	case in.moved:
		where += ` AND (j.state IS NULL OR j.category_id IS DISTINCT FROM p.category_id)`
	default:
		where += ` AND j.state IS NULL`
	}
	if len(in.exclude) > 0 && len(in.IDs) == 0 {
		args = append(args, pq.Array(in.exclude))
		where += ` AND NOT p.id = ANY($` + num(len(args)) + `)`
	}
	if in.noHint {
		where += ` AND COALESCE(i.specs, '') = '' AND COALESCE(i.user_explanation, '') = ''`
	}
	if in.needDetails && len(in.IDs) == 0 {
		where += ` AND (p.name ~ '[0-9]' OR COALESCE(i.specs, '') <> '' OR COALESCE(i.user_explanation, '') <> '')`
	}
	if in.CategoryID > 0 && len(in.IDs) == 0 {
		args = append(args, in.CategoryID)
		where += ` AND p.category_id IN (WITH RECURSIVE sub(id) AS (
			SELECT id FROM categories WHERE id = $` + num(len(args)) + `
			UNION SELECT c.id FROM categories c JOIN sub ON c.parent_id = sub.id) SELECT id FROM sub)`
	}
	from := ` FROM products p
		LEFT JOIN aicatalog_items i ON i.product_id = p.id
		LEFT JOIN aicatalog_job_items j ON j.product_id = p.id AND j.job = $1 WHERE ` + where
	var total int
	if err := p.store.db.GetContext(ctx, &total, `SELECT COUNT(*)`+from, args...); err != nil {
		return nil, 0, err
	}
	size := in.Size
	if size <= 0 {
		size = jobBatchDefault
	}
	size = min(size, jobBatchMax)
	if len(in.IDs) > 0 {
		size = jobBatchMax
	}
	if in.noCap {
		size = jobSelectMax
	}
	order := "p.id"
	if in.byName {
		order = "lower(p.name), p.id"
	}
	var rows []JobProduct
	err := p.store.db.SelectContext(ctx, &rows, `SELECT p.id, p.name, p.category_id,
		COALESCE(i.specs, '') AS specs, COALESCE(i.user_explanation, '') AS user_explanation,
		COALESCE(i.specs, '') <> '' OR COALESCE(i.user_explanation, '') <> '' AS has_hint`+from+` ORDER BY `+order+` LIMIT `+num(size), args...)
	if err != nil {
		return nil, 0, err
	}
	cats, err := p.store.Categories(ctx)
	if err != nil {
		return nil, 0, err
	}
	paths := pathsByID(cats)
	for i := range rows {
		rows[i].Category = paths[rows[i].CategoryID]
	}
	return rows, total, nil
}

// jobRedoOver starts a forced redo from the first product again.
func (h handlers) jobRedoOver(c echo.Context) error {
	j := h.p.job(c.Param("job"))
	if j == nil {
		return apperr.NotFound("job")
	}
	if _, err := h.p.store.db.ExecContext(c.Request().Context(), `DELETE FROM aicatalog_job_items WHERE job LIKE $1 || ':%redo'`, j.Key); err != nil {
		return apperr.Internal("start the redo over", err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) jobNext(c echo.Context) error {
	j := h.p.job(c.Param("job"))
	if j == nil {
		return apperr.NotFound("job")
	}
	var in JobRequest
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	ctx := c.Request().Context()
	s, err := h.settings(c)
	if err != nil {
		return err
	}
	var b Batch
	var total int
	if j.Select != nil {
		b, total, err = j.Select(ctx, h.p, in)
	} else {
		key := in.recordKey(j.Key)
		if in.Force {
			b.State = key
		}
		b.Products, total, err = selectProducts(ctx, h.p, key, in)
	}
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return err
		}
		return apperr.Internal("read the products", err)
	}
	b.Options, b.Settings, b.Redo = in.Options, s, len(in.IDs) > 0 || in.Force
	var out AskReply[JobPreview]
	out.Left = &total
	if len(b.Products) == 0 {
		out.Result = &JobPreview{Rows: []JobRow{}, Asked: []int64{}, Names: map[int64]string{}}
		return c.JSON(http.StatusOK, out)
	}
	system, user, err := j.Prompt(ctx, h.p, b)
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return err
		}
		return apperr.Internal("write the prompt", err)
	}
	full := withCustom(system, s.CustomPrompt) + "\n\n" + user + "\n\n" + jsonOnly
	reply := strings.TrimSpace(in.Reply)
	switch {
	case reply != "":
	case s.ManualMode:
		out.Prompt = full
		return c.JSON(http.StatusOK, out)
	default:
		if reply, err = NewClient(s).complete(ctx, system, user, false); err != nil {
			out.Prompt, out.Error = full, "The AI didn't answer ("+err.Error()+") — copy the prompt to a chatbot instead."
			return c.JSON(http.StatusOK, out)
		}
	}
	rows, err := j.Parse(ctx, h.p, b, reply)
	if err != nil {
		out.Prompt, out.Error = full, err.Error()
		return c.JSON(http.StatusOK, out)
	}
	pv := JobPreview{State: b.State, Rows: []JobRow{}, Asked: make([]int64, 0, len(b.Products)), Names: map[int64]string{}, Remaining: total - len(b.Products)}
	for _, x := range b.Products {
		pv.Asked = append(pv.Asked, x.ID)
		pv.Names[x.ID] = x.Name
	}
	for _, x := range b.Context {
		pv.Names[x.ID] = x.Name
	}
	for _, r := range rows {
		// A chatbot naming a product it wasn't asked about is dropped here, for every job.
		if _, asked := pv.Names[r.ProductID]; asked {
			pv.Rows = append(pv.Rows, r)
		}
	}
	out.Result = &pv
	return c.JSON(http.StatusOK, out)
}
