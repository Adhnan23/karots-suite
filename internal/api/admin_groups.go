package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/productgroups"

	"github.com/labstack/echo/v4"
)

// The till's menu cards (karots-pos "Cashier Menu", /admin/groups).
func (h *handlers) registerAdminGroups(g *echo.Group) {
	g.GET("/groups", h.AdminGroups)
	g.POST("/groups", h.AdminGroupCreate)
	g.PUT("/groups/:id", h.AdminGroupUpdate)
	g.DELETE("/groups/:id", h.AdminGroupDelete)
	g.POST("/groups/:id/move", h.AdminGroupMove)
	g.POST("/groups/:id/activate", h.AdminGroupReactivate)
	g.GET("/groups/:id/items", h.AdminGroupItems)
	g.POST("/groups/:id/items", h.AdminGroupItemAdd)
	g.DELETE("/groups/:id/items/:productId", h.AdminGroupItemRemove)
	g.PUT("/groups/:id/items/:productId/emoji", h.AdminGroupItemEmoji)
}

// AdminGroups is the whole tree depth-first; ?disabled=1 includes disabled groups.
func (h *handlers) AdminGroups(c echo.Context) error {
	rows, err := h.t.groups.Tree(c.Request().Context(), c.QueryParam("disabled") == "1")
	if err != nil {
		return err
	}
	kids := map[int64][]productgroups.Group{}
	var roots []productgroups.Group
	present := map[int64]bool{}
	for _, g := range rows {
		present[g.ID] = true
	}
	for _, g := range rows {
		if g.ParentID == nil || !present[*g.ParentID] { // a child of a hidden (disabled) parent shows at top
			roots = append(roots, g)
		} else {
			kids[*g.ParentID] = append(kids[*g.ParentID], g)
		}
	}
	out := make([]GroupNode, 0, len(rows))
	var walk func(g productgroups.Group, depth int)
	walk = func(g productgroups.Group, depth int) {
		var parent int64
		if g.ParentID != nil {
			parent = *g.ParentID
		}
		out = append(out, GroupNode{ID: g.ID, Name: g.Name, Emoji: strOr(g.Emoji), ParentID: parent, Depth: depth, IsActive: g.IsActive, ItemCount: g.ItemCount, ChildCount: len(kids[g.ID])})
		for _, k := range kids[g.ID] {
			walk(k, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminGroupCreate(c echo.Context) error {
	var req GroupRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := productgroups.CreateInput{Name: strings.TrimSpace(req.Name), Emoji: optStr(req.Emoji)}
	if req.ParentID > 0 {
		in.ParentID = &req.ParentID
	}
	id, err := h.t.groups.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product_group", strconv.FormatInt(id, 10), "created group "+in.Name)
	return c.JSON(http.StatusCreated, Option{ID: id, Name: in.Name})
}

func (h *handlers) AdminGroupUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req GroupRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := productgroups.UpdateInput{Name: strings.TrimSpace(req.Name), Emoji: optStr(req.Emoji)}
	if err := h.t.groups.Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product_group", strconv.FormatInt(id, 10), "updated group "+in.Name)
	return c.NoContent(http.StatusNoContent)
}

// AdminGroupDelete removes a group with its sub-groups and product links (the
// products themselves stay).
func (h *handlers) AdminGroupDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.groups.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "product_group", strconv.FormatInt(id, 10), "")
	return c.NoContent(http.StatusNoContent)
}

// AdminGroupMove swaps a group with its neighbour: ?dir=up|down.
func (h *handlers) AdminGroupMove(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.groups.Move(c.Request().Context(), id, c.QueryParam("dir")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminGroupReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.groups.Reactivate(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product_group", strconv.FormatInt(id, 10), "reactivated")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminGroupItems(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	rows, err := h.t.groups.Products(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := make([]GroupItem, 0, len(rows))
	for _, p := range rows {
		out = append(out, GroupItem{ProductID: p.ProductID, Name: p.Name, Barcode: strOr(p.Barcode), Emoji: strOr(p.Emoji), SellingPrice: dec2(p.SellingPrice),
			StockQty: p.StockQty.String(), UnitAbbr: p.UnitAbbr, IsService: p.IsService})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminGroupItemAdd(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req GroupItemRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if req.ProductID <= 0 {
		return apperr.Validation("choose a product to add")
	}
	if _, err := h.t.groups.Get(c.Request().Context(), id); err != nil {
		return err
	}
	if err := h.t.groups.LinkProduct(c.Request().Context(), id, req.ProductID, optStr(req.Emoji)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product_group", strconv.FormatInt(id, 10), "linked product "+strconv.FormatInt(req.ProductID, 10))
	return c.NoContent(http.StatusNoContent)
}

func groupItemIDs(c echo.Context) (int64, int64, error) {
	id, err := pathID(c)
	if err != nil {
		return 0, 0, err
	}
	pid, err := strconv.ParseInt(c.Param("productId"), 10, 64)
	if err != nil || pid <= 0 {
		return 0, 0, apperr.BadRequest("invalid product id")
	}
	return id, pid, nil
}

func (h *handlers) AdminGroupItemRemove(c echo.Context) error {
	id, pid, err := groupItemIDs(c)
	if err != nil {
		return err
	}
	if err := h.t.groups.UnlinkProduct(c.Request().Context(), id, pid); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminGroupItemEmoji(c echo.Context) error {
	id, pid, err := groupItemIDs(c)
	if err != nil {
		return err
	}
	var req GroupItemRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.t.groups.SetItemEmoji(c.Request().Context(), id, pid, optStr(req.Emoji)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
