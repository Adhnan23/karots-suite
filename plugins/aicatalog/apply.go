package aicatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"karots-suite/internal/features/categories"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// Run is one applied clean-up (a chunked session is one run).
type Run struct {
	ID      int64  `db:"id"`
	Summary string `db:"summary"`
}

var errNothingToUndo = errors.New("nothing to undo")

// ApplyPlan validates the ticked plan against real ids and applies it in one
// transaction, recording the exact undo ops (category moves, uncategorized
// placements and product renames). Invalid or unsafe ops (unknown
// ids, cycles, self-merge, a name another product has) are skipped, not fatal.
// Duplicates are never applied here — they are core merges, one by one.
//
// runID threads a chunked session into ONE run: 0 for the first chunk, the
// returned id for later ones, so a single undo rolls the whole session back.
func (s *Store) ApplyPlan(ctx context.Context, plan CleanupPlan, userID, runID int64) (string, int64, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()

	catExists := map[int64]bool{}
	prodExists := map[int64]bool{}
	for q, m := range map[string]map[int64]bool{`SELECT id FROM categories`: catExists, `SELECT id FROM products WHERE is_active`: prodExists} {
		var ids []int64
		if err := tx.SelectContext(ctx, &ids, q); err != nil {
			return "", 0, err
		}
		for _, id := range ids {
			m[id] = true
		}
	}

	var undo []map[string]any
	var nRen, nRep, nMerge, nMove, nUncat, nName int

	catRow := func(id int64) (string, *int64, error) {
		var r struct {
			Name     string `db:"name"`
			ParentID *int64 `db:"parent_id"`
		}
		err := tx.GetContext(ctx, &r, `SELECT name, parent_id FROM categories WHERE id=$1`, id)
		return r.Name, r.ParentID, err
	}
	// move puts a product in a category, recording where it was.
	move := func(pid, cat int64) (bool, error) {
		var oldCat int64
		if err := tx.GetContext(ctx, &oldCat, `SELECT category_id FROM products WHERE id=$1`, pid); err != nil || oldCat == cat {
			return false, nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET category_id=$2 WHERE id=$1`, pid, cat); err != nil {
			return false, err
		}
		undo = append(undo, map[string]any{"kind": "move_product", "id": pid, "category_id": oldCat})
		return true, nil
	}

	for _, op := range plan.Renames {
		to := strings.TrimSpace(op.To)
		if !catExists[op.ID] || to == "" || len(to) > 80 {
			continue
		}
		old, _, err := catRow(op.ID)
		if err != nil || old == to {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE categories SET name=$2 WHERE id=$1`, op.ID, to); err != nil {
			return "", 0, err
		}
		undo = append(undo, map[string]any{"kind": "rename", "id": op.ID, "name": old})
		nRen++
	}

	// Reparents (no cycles: the new parent must not be the node or below it).
	for _, op := range plan.Reparents {
		if !catExists[op.ID] {
			continue
		}
		var newParent *int64
		if op.ParentID != 0 {
			if !catExists[op.ParentID] || op.ParentID == op.ID {
				continue
			}
			if desc, err := isDescendant(ctx, tx, op.ID, op.ParentID); err != nil {
				return "", 0, err
			} else if desc {
				continue
			}
			p := op.ParentID
			newParent = &p
		}
		_, oldParent, err := catRow(op.ID)
		if err != nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE categories SET parent_id=$2 WHERE id=$1`, op.ID, newParent); err != nil {
			return "", 0, err
		}
		undo = append(undo, map[string]any{"kind": "reparent", "id": op.ID, "parent_id": oldParent})
		nRep++
	}

	// Merges (from -> into): move products & child categories, let plugins
	// (CategoryMerger) re-point their own references, then delete from.
	for _, op := range plan.Merges {
		if !catExists[op.From] || !catExists[op.Into] || op.From == op.Into {
			continue
		}
		if desc, err := isDescendant(ctx, tx, op.From, op.Into); err != nil {
			return "", 0, err
		} else if desc {
			continue // into sits below from: re-parenting from's children onto it would loop
		}
		name, parent, err := catRow(op.From)
		if err != nil {
			continue
		}
		var prodIDs, childIDs []int64
		if err := tx.SelectContext(ctx, &prodIDs, `SELECT id FROM products WHERE category_id=$1`, op.From); err != nil {
			return "", 0, err
		}
		if err := tx.SelectContext(ctx, &childIDs, `SELECT id FROM categories WHERE parent_id=$1`, op.From); err != nil {
			return "", 0, err
		}
		for _, q := range []string{
			`UPDATE products SET category_id=$2 WHERE category_id=$1`,
			`UPDATE categories SET parent_id=$2 WHERE parent_id=$1`,
		} {
			if _, err := tx.ExecContext(ctx, q, op.From, op.Into); err != nil {
				return "", 0, err
			}
		}
		// Plugins that keep category ids re-point them; their notes ride the undo.
		notes := map[string]json.RawMessage{}
		for _, m := range plugin.CategoryMergers() {
			note, err := m.Merge(ctx, tx, op.From, op.Into)
			if err != nil {
				return "", 0, fmt.Errorf("%s: %w", m.Key, err)
			}
			if note != nil {
				notes[m.Key] = note
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE id=$1`, op.From); err != nil {
			return "", 0, err
		}
		catExists[op.From] = false
		undo = append(undo, map[string]any{
			"kind": "unmerge", "id": op.From, "name": name, "parent_id": parent,
			"products": prodIDs, "children": childIDs, "plugins": notes,
		})
		nMerge++
	}

	for _, op := range plan.Moves {
		if !prodExists[op.ProductID] || !catExists[op.CategoryID] {
			continue
		}
		if ok, err := move(op.ProductID, op.CategoryID); err != nil {
			return "", 0, err
		} else if ok {
			nMove++
		}
	}

	// Uncategorized: find or create the path (new levels are undone if still empty).
	cats := categories.NewService(tx)
	for _, op := range plan.Uncategorized {
		path := normPath(op.Category)
		if !prodExists[op.ProductID] || path == "" {
			continue
		}
		var maxBefore int64
		if err := tx.GetContext(ctx, &maxBefore, `SELECT COALESCE(max(id), 0) FROM categories`); err != nil {
			return "", 0, err
		}
		cat, err := cats.FindOrCreateByPath(ctx, path)
		if err != nil {
			return "", 0, err
		}
		var created []int64
		if err := tx.SelectContext(ctx, &created, `SELECT id FROM categories WHERE id > $1 ORDER BY id`, maxBefore); err != nil {
			return "", 0, err
		}
		for _, id := range created {
			catExists[id] = true
			undo = append(undo, map[string]any{"kind": "created_category", "id": id})
		}
		if ok, err := move(op.ProductID, cat); err != nil {
			return "", 0, err
		} else if ok {
			nUncat++
		}
	}

	for _, op := range plan.Names {
		to := strings.TrimSpace(op.To)
		if !prodExists[op.ProductID] || to == "" || len(to) > 150 {
			continue
		}
		var old string
		var taken bool
		if err := tx.GetContext(ctx, &old, `SELECT name FROM products WHERE id=$1`, op.ProductID); err != nil || old == to {
			continue
		}
		if err := tx.GetContext(ctx, &taken, `SELECT EXISTS (SELECT 1 FROM products WHERE is_active AND id <> $1 AND lower(name) = lower($2))`, op.ProductID, to); err != nil {
			return "", 0, err
		}
		if taken {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET name=$2 WHERE id=$1`, op.ProductID, to); err != nil {
			return "", 0, err
		}
		undo = append(undo, map[string]any{"kind": "rename_product", "id": op.ProductID, "name": old})
		nName++
	}

	if len(undo) == 0 {
		return "No changes applied.", runID, tx.Commit()
	}
	summary := summaryText(nRen, nRep, nMerge, nMove, nUncat, nName)

	if runID != 0 {
		// Append to the session's run: reverse replay undoes the latest first.
		var run struct {
			Undo    []byte `db:"undo"`
			Summary string `db:"summary"`
		}
		if err := tx.GetContext(ctx, &run,
			`SELECT undo, summary FROM aicatalog_optimize_runs WHERE id=$1 AND reverted_at IS NULL FOR UPDATE`, runID); err != nil {
			return "", 0, err
		}
		var prev []map[string]any
		_ = json.Unmarshal(run.Undo, &prev)
		merged, _ := json.Marshal(append(prev, undo...))
		combined := run.Summary + "; " + summary
		if _, err := tx.ExecContext(ctx,
			`UPDATE aicatalog_optimize_runs SET summary=$2, undo=$3 WHERE id=$1`, runID, combined, merged); err != nil {
			return "", 0, err
		}
		return combined, runID, tx.Commit()
	}

	undoJSON, _ := json.Marshal(undo)
	var newID int64
	if err := tx.GetContext(ctx, &newID,
		`INSERT INTO aicatalog_optimize_runs (applied_by, summary, undo) VALUES ($1,$2,$3) RETURNING id`,
		userID, summary, undoJSON); err != nil {
		return "", 0, err
	}
	return summary, newID, tx.Commit()
}

func summaryText(ren, rep, merge, move, uncat, name int) string {
	var parts []string
	for _, x := range []struct {
		n    int
		what string
	}{{ren, "category renamed"}, {rep, "category moved"}, {merge, "category merged"}, {move, "product moved"}, {uncat, "uncategorized placed"}, {name, "product renamed"}} {
		if x.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", x.n, x.what))
		}
	}
	return strings.Join(parts, ", ")
}

// isDescendant reports whether `node` is `ancestor` or sits below it, walking up
// from node's candidate parent. Used to stop a reparent from forming a cycle.
func isDescendant(ctx context.Context, tx *sqlx.Tx, ancestor, node int64) (bool, error) {
	cur := &node
	for i := 0; i < 100 && cur != nil; i++ {
		if *cur == ancestor {
			return true, nil
		}
		var next *int64
		if err := tx.GetContext(ctx, &next, `SELECT parent_id FROM categories WHERE id=$1`, *cur); err != nil {
			return false, err
		}
		cur = next
	}
	return false, nil
}

// LatestRun returns the most recent un-reverted run, or nil if none.
func (s *Store) LatestRun(ctx context.Context) (*Run, error) {
	var r Run
	err := s.db.GetContext(ctx, &r,
		`SELECT id, summary FROM aicatalog_optimize_runs WHERE reverted_at IS NULL ORDER BY id DESC LIMIT 1`)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// Revert replays the undo ops of the latest un-reverted run, restoring the prior
// category tree and product placements, then marks the run reverted.
func (s *Store) Revert(ctx context.Context) (string, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var run struct {
		ID   int64  `db:"id"`
		Sum  string `db:"summary"`
		Undo []byte `db:"undo"`
	}
	err = tx.GetContext(ctx, &run,
		`SELECT id, summary, undo FROM aicatalog_optimize_runs WHERE reverted_at IS NULL ORDER BY id DESC LIMIT 1`)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return "", errNothingToUndo
		}
		return "", err
	}
	var ops []map[string]any
	if err := json.Unmarshal(run.Undo, &ops); err != nil {
		return "", err
	}
	// Replay in reverse of how they were applied.
	for i := len(ops) - 1; i >= 0; i-- {
		if err := revertOp(ctx, tx, ops[i]); err != nil {
			return "", err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE aicatalog_optimize_runs SET reverted_at=now() WHERE id=$1`, run.ID); err != nil {
		return "", err
	}
	return run.Sum, tx.Commit()
}

func revertOp(ctx context.Context, tx *sqlx.Tx, op map[string]any) error {
	switch op["kind"] {
	case "rename":
		_, err := tx.ExecContext(ctx, `UPDATE categories SET name=$2 WHERE id=$1`, toInt(op["id"]), op["name"])
		return err
	case "reparent":
		_, err := tx.ExecContext(ctx, `UPDATE categories SET parent_id=$2 WHERE id=$1`, toInt(op["id"]), toNullInt(op["parent_id"]))
		return err
	case "move_product":
		_, err := tx.ExecContext(ctx, `UPDATE products SET category_id=$2 WHERE id=$1`, toInt(op["id"]), toInt(op["category_id"]))
		return err
	case "rename_product":
		_, err := tx.ExecContext(ctx, `UPDATE products SET name=$2 WHERE id=$1`, toInt(op["id"]), op["name"])
		return err
	case "created_category":
		// Only while nothing has been put in it since.
		_, err := tx.ExecContext(ctx, `DELETE FROM categories c WHERE c.id=$1
			AND NOT EXISTS (SELECT 1 FROM products WHERE category_id = c.id)
			AND NOT EXISTS (SELECT 1 FROM categories WHERE parent_id = c.id)`, toInt(op["id"]))
		return err
	case "unmerge":
		id := toInt(op["id"])
		// Recreate the deleted category with its original id.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO categories (id, name, parent_id) VALUES ($1,$2,$3) ON CONFLICT (id) DO NOTHING`,
			id, op["name"], toNullInt(op["parent_id"])); err != nil {
			return err
		}
		for _, pid := range toIntSlice(op["products"]) {
			if _, err := tx.ExecContext(ctx, `UPDATE products SET category_id=$2 WHERE id=$1`, pid, id); err != nil {
				return err
			}
		}
		for _, cid := range toIntSlice(op["children"]) {
			if _, err := tx.ExecContext(ctx, `UPDATE categories SET parent_id=$2 WHERE id=$1`, cid, id); err != nil {
				return err
			}
		}
		if err := unmergePlugins(ctx, tx, op["plugins"]); err != nil {
			return err
		}
		// Keep the id sequence ahead of any reused id.
		_, _ = tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('categories','id'), (SELECT max(id) FROM categories))`)
		return nil
	}
	return nil
}

// JSON numbers decode as float64; these coerce them back to ints for SQL.
func toInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	}
	return 0
}

func toNullInt(v any) any {
	if v == nil {
		return nil
	}
	return toInt(v)
}

func toIntSlice(v any) []int64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(arr))
	for _, e := range arr {
		out = append(out, toInt(e))
	}
	return out
}

// unmergePlugins hands each plugin its note from the merge (a run applied before
// a plugin was added has none for it; a plugin since removed is skipped).
func unmergePlugins(ctx context.Context, tx *sqlx.Tx, v any) error {
	notes, _ := v.(map[string]any)
	for _, m := range plugin.CategoryMergers() {
		note, ok := notes[m.Key]
		if !ok {
			continue
		}
		raw, _ := json.Marshal(note)
		if err := m.Unmerge(ctx, tx, raw); err != nil {
			return fmt.Errorf("%s: %w", m.Key, err)
		}
	}
	return nil
}
