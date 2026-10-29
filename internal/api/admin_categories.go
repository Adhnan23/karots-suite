package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/categories"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerAdminCategories(g *echo.Group) {
	g.GET("/categories", h.AdminCategories)
	g.POST("/categories", h.AdminCategoryCreate)
	g.POST("/categories/quick", h.AdminCategoryQuick)
	g.PUT("/categories/:id", h.AdminCategoryUpdate)
	g.DELETE("/categories/:id", h.AdminCategoryDelete)
}

func (h *handlers) categoryTree(c echo.Context) ([]CategoryNode, error) {
	ctx := c.Request().Context()
	tree, err := categories.NewService(h.d.DB).Tree(ctx)
	if err != nil {
		return nil, err
	}
	var counts []struct {
		ID int64 `db:"category_id"`
		N  int   `db:"n"`
	}
	if err := h.d.DB.SelectContext(ctx, &counts, `SELECT category_id, COUNT(*) AS n FROM products WHERE is_active GROUP BY category_id`); err != nil {
		return nil, apperr.Internal("failed to count products per category", err)
	}
	per := make(map[int64]int, len(counts))
	for _, r := range counts {
		per[r.ID] = r.N
	}
	out := make([]CategoryNode, 0, len(tree))
	for _, n := range tree {
		var parent int64
		if n.ParentID != nil {
			parent = *n.ParentID
		}
		path := n.Path
		if path == nil {
			path = []int64{}
		}
		out = append(out, CategoryNode{ID: n.ID, Name: n.Name, ParentID: parent, Depth: n.Depth, ChildCount: n.ChildCount, Path: path, ProductCount: per[n.ID]})
	}
	return out, nil
}

// AdminCategories is the whole tree, depth-first — small enough to send at once.
func (h *handlers) AdminCategories(c echo.Context) error {
	out, err := h.categoryTree(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func bindCategory(c echo.Context) (string, *int64, error) {
	var req CategoryRequest
	if err := c.Bind(&req); err != nil {
		return "", nil, apperr.BadRequest("invalid request body")
	}
	name, err := categories.CleanName(req.Name)
	if err != nil {
		return "", nil, err
	}
	if req.ParentID <= 0 {
		return name, nil, nil
	}
	return name, &req.ParentID, nil
}

func (h *handlers) AdminCategoryCreate(c echo.Context) error {
	name, parent, err := bindCategory(c)
	if err != nil {
		return err
	}
	cat, err := categories.NewService(h.d.DB).Create(c.Request().Context(), categories.CreateInput{Name: name, ParentID: parent})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "category", strconv.FormatInt(cat.ID, 10), "created category "+name)
	return c.JSON(http.StatusCreated, CategoryQuickResult{ID: cat.ID, Name: cat.Name})
}

// AdminCategoryQuick is the pickers' inline create: find-or-create name under
// parent, so a repeat picks the existing category instead of duplicating it.
func (h *handlers) AdminCategoryQuick(c echo.Context) error {
	name, parent, err := bindCategory(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	path, depth := name, 0
	if parent != nil {
		crumbs, err := h.t.products.CategoryPath(ctx, *parent)
		if err != nil {
			return err
		}
		if len(crumbs) == 0 {
			return apperr.NotFound("parent category")
		}
		parts := make([]string, 0, len(crumbs)+1)
		for _, cr := range crumbs {
			parts = append(parts, cr.Name)
		}
		path, depth = strings.Join(append(parts, name), " > "), len(crumbs)
	}
	id, err := categories.NewService(h.d.DB).FindOrCreateByPath(ctx, path)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "category", strconv.FormatInt(id, 10), "created from picker: "+path)
	return c.JSON(http.StatusCreated, CategoryQuickResult{ID: id, Name: name, Depth: depth})
}

func (h *handlers) AdminCategoryUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	name, parent, err := bindCategory(c)
	if err != nil {
		return err
	}
	if parent != nil {
		// A category can't sit under itself or its own sub-category: that makes a
		// loop the tree walk never reaches, and the branch vanishes from every picker.
		tree, err := h.categoryTree(c)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(tree, func(n CategoryNode) bool { return n.ID == *parent })
		if i < 0 {
			return apperr.NotFound("parent category")
		}
		if *parent == id || slices.Contains(tree[i].Path, id) {
			return apperr.Validation("a category can't go inside itself or one of its own sub-categories")
		}
	}
	if err := categories.NewService(h.d.DB).Update(c.Request().Context(), id, categories.UpdateInput{Name: name, ParentID: parent}); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "category", strconv.FormatInt(id, 10), "updated category "+name)
	return c.NoContent(http.StatusNoContent)
}

// AdminCategoryDelete refuses while products are filed here; sub-categories
// move up to the top level.
func (h *handlers) AdminCategoryDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := categories.NewService(h.d.DB).Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "category", strconv.FormatInt(id, 10), "deleted category")
	return c.NoContent(http.StatusNoContent)
}
