package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/reorder"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

func (h *handlers) registerAdminReorderLevels(g *echo.Group) {
	g.POST("/products/reorder-level", h.AdminReorderLevel)
	g.POST("/products/reorder-auto", h.AdminReorderAuto)
}

// AdminReorderLevel acts on Fixed levels that look off: "suggested" sets each to
// what sales suggest (Use N / Use all), "auto" switches them to Auto instead.
func (h *handlers) AdminReorderLevel(c echo.Context) error {
	var req ReorderLevelRequest
	if err := c.Bind(&req); err != nil || len(req.IDs) == 0 {
		return apperr.BadRequest("pick the products")
	}
	ctx := c.Request().Context()
	switch req.Use {
	case "suggested":
		if _, err := h.d.DB.ExecContext(ctx, `
			UPDATE products SET reorder_level = (reorder_calc->>'calcLevel')::int
			WHERE id = ANY($1) AND NOT reorder_auto AND reorder_calc IS NOT NULL`, pq.Array(req.IDs)); err != nil {
			return apperr.Internal("failed to set reorder levels", err)
		}
		h.audit(c, audit.ActionUpdate, "product", "", "set "+strconv.Itoa(len(req.IDs))+" reorder levels to what sales suggest")
	case "auto":
		if err := h.switchAuto(c, req.IDs); err != nil {
			return err
		}
		h.audit(c, audit.ActionUpdate, "product", "", "switched "+strconv.Itoa(len(req.IDs))+" reorder levels to Auto")
	default:
		return apperr.Validation("use is suggested or auto")
	}
	return c.JSON(http.StatusOK, ReorderAutoResult{Switched: len(req.IDs)})
}

// AdminReorderAuto switches every Fixed product of a category (and its
// sub-categories), or of the whole shop, to Auto.
func (h *handlers) AdminReorderAuto(c echo.Context) error {
	var req ReorderAutoRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	var ids []int64
	if err := h.d.DB.SelectContext(c.Request().Context(), &ids, `
		WITH RECURSIVE subcats AS (
			SELECT id FROM categories WHERE id = $1
			UNION ALL SELECT c.id FROM categories c JOIN subcats sc ON c.parent_id = sc.id)
		SELECT id FROM products WHERE is_active AND NOT is_service AND NOT reorder_auto
		  AND ($1 = 0 OR category_id IN (SELECT id FROM subcats))`, req.CategoryID); err != nil {
		return apperr.Internal("failed to find products", err)
	}
	if err := h.switchAuto(c, ids); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", "", "switched "+strconv.Itoa(len(ids))+" reorder levels to Auto")
	return c.JSON(http.StatusOK, ReorderAutoResult{Switched: len(ids)})
}

// switchAuto makes Fixed products Auto, starting from their current number,
// and learns their level now rather than tonight.
func (h *handlers) switchAuto(c echo.Context, ids []int64) error {
	ctx := c.Request().Context()
	if _, err := h.d.DB.ExecContext(ctx, `UPDATE products SET reorder_auto = true, reorder_start = reorder_level
		WHERE id = ANY($1) AND NOT reorder_auto`, pq.Array(ids)); err != nil {
		return apperr.Internal("failed to switch to Auto", err)
	}
	if err := reorder.RunIDs(ctx, h.d.DB, time.Now(), ids); err != nil {
		return apperr.Internal("failed to learn reorder levels", err)
	}
	return nil
}

// relearn recalculates a product saved as Auto, so the form shows its level now.
func (h *handlers) relearn(c echo.Context, p *products.Product) {
	if p != nil && p.ReorderAuto && !p.IsService {
		_ = reorder.RunOne(c.Request().Context(), h.d.DB, p.ID) // best effort: tonight's run catches up
	}
}
