package aicatalog

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// cleanupChunk is how many products go into one clean-up pass, so a big
// catalogue is tidied in pieces small enough to paste into a chatbot.
const cleanupChunk = 150

// uncatSQL: core files a product with no category under "Uncategorized".
const uncatSQL = `EXISTS (SELECT 1 FROM categories u WHERE u.id = p.category_id AND lower(u.name) = 'uncategorized')`

func (s *Store) CleanupProducts(ctx context.Context, limit, offset int) ([]CleanupProduct, error) {
	var rows []CleanupProduct
	err := s.db.SelectContext(ctx, &rows, `SELECT p.id, p.name, p.category_id, `+uncatSQL+` AS uncat
		FROM products p WHERE p.is_active AND p.is_service = false
		ORDER BY p.id LIMIT $1 OFFSET $2`, limit, offset)
	return rows, err
}

func (s *Store) CountCleanupProducts(ctx context.Context) (int, error) {
	var n int
	err := s.db.GetContext(ctx, &n, `SELECT count(*) FROM products WHERE is_active AND is_service = false`)
	return n, err
}

// SalesCounts: how many sales each product appears on (to pick which of a look-alike pair to keep).
func (s *Store) SalesCounts(ctx context.Context, ids []int64) (map[int64]int, error) {
	var rows []struct {
		ID int64 `db:"product_id"`
		N  int   `db:"n"`
	}
	err := s.db.SelectContext(ctx, &rows, `SELECT product_id, count(DISTINCT sale_id) AS n FROM sale_items
		WHERE product_id = ANY($1) GROUP BY product_id`, pq.Array(ids))
	out := map[int64]int{}
	for _, r := range rows {
		out[r.ID] = r.N
	}
	return out, err
}

// EmptyCategory is a category with no products (any) and no children.
type EmptyCategory struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

const emptySQL = `NOT EXISTS (SELECT 1 FROM products WHERE category_id = c.id)
	AND NOT EXISTS (SELECT 1 FROM categories k WHERE k.parent_id = c.id)`

func (s *Store) EmptyCategories(ctx context.Context) ([]EmptyCategory, error) {
	out := []EmptyCategory{}
	err := s.db.SelectContext(ctx, &out, `SELECT c.id, c.name FROM categories c WHERE `+emptySQL+` ORDER BY c.name`)
	return out, err
}

// DeleteEmpty deletes the listed categories that are still empty.
func (s *Store) DeleteEmpty(ctx context.Context, ids []int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM categories c WHERE c.id = ANY($1) AND `+emptySQL, pq.Array(ids))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DupProduct / DupPair: a look-alike pair, the one with more sales kept.
type DupProduct struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Sales int    `json:"sales"`
}

type DupPair struct {
	Keep   DupProduct `json:"keep"`
	Drop   DupProduct `json:"drop"`
	Reason string     `json:"reason"`
}

// CleanupPreview is one chunk's plan with the names the page shows.
type CleanupPreview struct {
	Plan       CleanupPlan      `json:"plan"`
	Categories map[int64]string `json:"categories"` // id → "A > B"
	Products   map[int64]string `json:"products"`
	Duplicates []DupPair        `json:"duplicates"`
	Chunk      int              `json:"chunk"`
	Chunks     int              `json:"chunks"`
	Next       int              `json:"next"` // offset of the next chunk, -1 when this is the last
}

type CleanupRequest struct {
	Offset int    `json:"offset"`
	Reply  string `json:"reply"`
}

type ApplyRequest struct {
	Plan  CleanupPlan `json:"plan"`
	RunID int64       `json:"runId"`
}

type ApplyReply struct {
	Summary string `json:"summary"`
	RunID   int64  `json:"runId"`
}

func (h handlers) cleanupRoutes(a plugin.Group) {
	a.POST("/aicatalog/cleanup", h.cleanup)
	a.POST("/aicatalog/cleanup/apply", h.apply)
	a.GET("/aicatalog/cleanup/last", h.last)
	a.POST("/aicatalog/cleanup/undo", h.undo)
	a.GET("/aicatalog/cleanup/empty-categories", h.emptyCategories)
	a.POST("/aicatalog/cleanup/empty-categories", h.deleteEmpty)
}

func (h handlers) cleanup(c echo.Context) error {
	var in CleanupRequest
	if err := c.Bind(&in); err != nil || in.Offset < 0 {
		return apperr.Validation("invalid request")
	}
	ctx := c.Request().Context()
	s, err := h.settings(c)
	if err != nil {
		return err
	}
	cats, err := h.p.store.Categories(ctx)
	if err != nil {
		return apperr.Internal("read categories", err)
	}
	paths := pathsByID(cats)
	total, err := h.p.store.CountCleanupProducts(ctx)
	if err != nil {
		return apperr.Internal("count products", err)
	}
	prods, err := h.p.store.CleanupProducts(ctx, cleanupChunk, in.Offset)
	if err != nil {
		return apperr.Internal("read products", err)
	}
	full := func() string { return CleanupPrompt(cats, paths, prods, s) }
	var plan CleanupPlan
	var out AskReply[CleanupPreview]
	switch {
	case strings.TrimSpace(in.Reply) != "":
		if plan, err = parseCleanup(in.Reply); err != nil {
			out.Prompt, out.Error = full(), err.Error()
			return c.JSON(http.StatusOK, out)
		}
	case s.ManualMode:
		out.Prompt = full()
		return c.JSON(http.StatusOK, out)
	default:
		if plan, err = NewClient(s).Cleanup(ctx, cats, paths, prods); err != nil {
			out.Prompt, out.Error = full(), "The AI didn't answer ("+err.Error()+") — copy the prompt to a chatbot instead."
			return c.JSON(http.StatusOK, out)
		}
	}
	pv, err := h.preview(ctx, plan, paths, prods)
	if err != nil {
		return err
	}
	pv.Chunk, pv.Chunks, pv.Next = in.Offset/cleanupChunk+1, max(1, (total+cleanupChunk-1)/cleanupChunk), -1
	if in.Offset+cleanupChunk < total {
		pv.Next = in.Offset + cleanupChunk
	}
	out.Result = &pv
	return c.JSON(http.StatusOK, out)
}

// preview drops every op naming a category or an active product that doesn't
// exist (so the page never lists a change apply would skip) and orders look-alike
// pairs. A product outside this chunk is fine — a reply pasted from another part
// still applies.
func (h handlers) preview(ctx context.Context, plan CleanupPlan, paths map[int64]string, prods []CleanupProduct) (CleanupPreview, error) {
	names := map[int64]string{}
	for _, p := range prods {
		names[p.ID] = p.Name
	}
	if err := h.p.store.ProductNames(ctx, referenced(plan), names); err != nil {
		return CleanupPreview{}, apperr.Internal("read products", err)
	}
	cat := func(id int64) bool { _, ok := paths[id]; return ok }
	prod := func(id int64) bool { _, ok := names[id]; return ok }
	var out CleanupPlan
	for _, o := range plan.Renames {
		if cat(o.ID) && strings.TrimSpace(o.To) != "" {
			out.Renames = append(out.Renames, o)
		}
	}
	for _, o := range plan.Reparents {
		if cat(o.ID) && (o.ParentID == 0 || cat(o.ParentID)) && o.ID != o.ParentID {
			out.Reparents = append(out.Reparents, o)
		}
	}
	for _, o := range plan.Merges {
		if cat(o.From) && cat(o.Into) && o.From != o.Into {
			out.Merges = append(out.Merges, o)
		}
	}
	for _, o := range plan.Moves {
		if prod(o.ProductID) && cat(o.CategoryID) {
			out.Moves = append(out.Moves, o)
		}
	}
	for _, o := range plan.Uncategorized {
		if prod(o.ProductID) && o.Category != "" {
			out.Uncategorized = append(out.Uncategorized, o)
		}
	}
	for _, o := range plan.Names {
		if prod(o.ProductID) && strings.TrimSpace(o.To) != "" && strings.TrimSpace(o.To) != names[o.ProductID] {
			out.Names = append(out.Names, o)
		}
	}
	var ids []int64
	for _, d := range plan.Duplicates {
		ids = append(ids, d.A, d.B)
	}
	sales, err := h.p.store.SalesCounts(ctx, ids)
	if err != nil {
		return CleanupPreview{}, apperr.Internal("count sales", err)
	}
	dups := []DupPair{}
	seen := map[[2]int64]bool{}
	for _, d := range plan.Duplicates {
		if !prod(d.A) || !prod(d.B) || d.A == d.B || seen[[2]int64{min(d.A, d.B), max(d.A, d.B)}] {
			continue
		}
		seen[[2]int64{min(d.A, d.B), max(d.A, d.B)}] = true
		a, b := DupProduct{d.A, names[d.A], sales[d.A]}, DupProduct{d.B, names[d.B], sales[d.B]}
		if b.Sales > a.Sales || (b.Sales == a.Sales && b.ID < a.ID) {
			a, b = b, a
		}
		dups = append(dups, DupPair{Keep: a, Drop: b, Reason: d.Reason})
	}
	return CleanupPreview{Plan: out, Categories: paths, Products: names, Duplicates: dups}, nil
}

// referenced lists every product id the plan names.
func referenced(p CleanupPlan) []int64 {
	var ids []int64
	for _, o := range p.Moves {
		ids = append(ids, o.ProductID)
	}
	for _, o := range p.Uncategorized {
		ids = append(ids, o.ProductID)
	}
	for _, o := range p.Names {
		ids = append(ids, o.ProductID)
	}
	for _, d := range p.Duplicates {
		ids = append(ids, d.A, d.B)
	}
	return ids
}

// ProductNames adds the names of the active, non-service products among ids to names.
func (s *Store) ProductNames(ctx context.Context, ids []int64, names map[int64]string) error {
	var rows []struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT id, name FROM products
		WHERE id = ANY($1) AND is_active AND is_service = false`, pq.Array(ids)); err != nil {
		return err
	}
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	return nil
}

func pathsByID(rows []CatRow) map[int64]string {
	byID := make(map[int64]CatRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		path, cur := r.Name, r
		for i := 0; i < 20 && cur.ParentID != nil; i++ {
			p, ok := byID[*cur.ParentID]
			if !ok {
				break
			}
			path, cur = p.Name+" > "+path, p
		}
		out[r.ID] = path
	}
	return out
}

func (h handlers) apply(c echo.Context) error {
	var in ApplyRequest
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid plan")
	}
	in.Plan.Duplicates = nil
	summary, run, err := h.p.store.ApplyPlan(c.Request().Context(), in.Plan, middleware.CurrentUserID(c), in.RunID)
	if err != nil {
		return apperr.Internal("apply the clean-up", err)
	}
	return c.JSON(http.StatusOK, ApplyReply{Summary: summary, RunID: run})
}

func (h handlers) last(c echo.Context) error {
	r, err := h.p.store.LatestRun(c.Request().Context())
	if err != nil {
		return apperr.Internal("read the last clean-up", err)
	}
	return c.JSON(http.StatusOK, map[string]*Run{"run": r})
}

func (h handlers) undo(c echo.Context) error {
	summary, err := h.p.store.Revert(c.Request().Context())
	if errors.Is(err, errNothingToUndo) {
		return apperr.Validation("There is no clean-up to undo.")
	}
	if err != nil {
		return apperr.Internal("undo the clean-up", err)
	}
	return c.JSON(http.StatusOK, map[string]string{"summary": summary})
}

func (h handlers) emptyCategories(c echo.Context) error {
	rows, err := h.p.store.EmptyCategories(c.Request().Context())
	if err != nil {
		return apperr.Internal("list empty categories", err)
	}
	return c.JSON(http.StatusOK, map[string][]EmptyCategory{"categories": rows})
}

func (h handlers) deleteEmpty(c echo.Context) error {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.Bind(&in); err != nil || len(in.IDs) == 0 {
		return apperr.Validation("pick the categories to delete")
	}
	n, err := h.p.store.DeleteEmpty(c.Request().Context(), in.IDs)
	if err != nil {
		return apperr.Internal("delete empty categories", err)
	}
	return c.JSON(http.StatusOK, map[string]int64{"deleted": n})
}
