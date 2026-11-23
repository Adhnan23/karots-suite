package api

import (
	"fmt"
	"net/http"
	"sort"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/categories"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// Delete empty categories (docs/rules/stock.md CAT-1, 2): the list of categories
// with no products and no sub-categories, and deleting the ones the user ticked —
// each checked again at that moment, so one that filled up meanwhile is kept.

const emptyCategoryWhere = `NOT EXISTS (SELECT 1 FROM products p WHERE p.category_id = c.id)
	AND NOT EXISTS (SELECT 1 FROM categories k WHERE k.parent_id = c.id)`

func (h *handlers) AdminCategoriesEmpty(c echo.Context) error {
	ctx := c.Request().Context()
	var ids []int64
	if err := h.d.DB.SelectContext(ctx, &ids, `SELECT c.id FROM categories c WHERE `+emptyCategoryWhere); err != nil {
		return err
	}
	paths, err := categoryPaths(ctx, h.d.DB)
	if err != nil {
		return err
	}
	out := make([]EmptyCategory, 0, len(ids))
	for _, id := range ids {
		out = append(out, EmptyCategory{ID: id, Path: paths[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminCategoriesDeleteEmpty(c echo.Context) error {
	ctx := c.Request().Context()
	var req DeleteEmptyCategoriesRequest
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return apperr.BadRequest("tick the categories to delete")
	}
	paths, err := categoryPaths(ctx, h.d.DB)
	if err != nil {
		return err
	}
	res := DeleteEmptyCategoriesResult{Kept: []string{}}
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		tctx := db.InTx(ctx, tx)
		for _, id := range req.IDs {
			gone := false
			if err := db.WithTx(tctx, h.d.DB, func(tx *sqlx.Tx) error { // a savepoint each: one refused, the rest go on
				var empty bool
				if err := tx.GetContext(ctx, &empty, `SELECT EXISTS (SELECT 1 FROM categories c WHERE c.id = $1 AND `+emptyCategoryWhere+`)`, id); err != nil || !empty {
					return err
				}
				if err := categories.NewService(tx).Delete(ctx, id); err != nil {
					return err
				}
				gone = true
				return nil
			}); err != nil {
				if _, refused := apperr.As(err); !refused { // a refusal (in use) keeps it; anything else stops
					return err
				}
			}
			if gone {
				res.Deleted++
			} else {
				res.Kept = append(res.Kept, fmt.Sprintf("%s — it has products or sub-categories now", paths[id]))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "category", "", fmt.Sprintf("deleted %d empty categories", res.Deleted))
	return c.JSON(http.StatusOK, res)
}
