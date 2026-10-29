package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const reorderPageSize = 30

func (h *handlers) registerAdminReorder(g *echo.Group) {
	g.GET("/reorder", h.AdminReorder) // builds with POST /purchases/draft (admin_purchases.go)
}

// AdminReorder is the low-stock worklist with a demand hint per product:
// search, category (with its
// sub-categories), preferred supplier; ?all=1 every match on one page;
// ?looksOff=1 the Fixed levels far from what sales suggest, low or not.
func (h *handlers) AdminReorder(c echo.Context) error {
	out, err := h.buildReorderList(c, true)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

// walkLow returns every product the query matches, a full page at a time
// (List caps one call at MaxListLimit).
func (h *handlers) walkLow(ctx context.Context, q products.ListQuery) ([]products.Product, error) {
	var rows []products.Product
	q.Limit = products.MaxListLimit
	for q.Page = 1; ; q.Page++ {
		got, n, err := h.t.products.List(ctx, q)
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
		if len(got) == 0 || len(rows) >= n {
			return rows, nil
		}
	}
}

// reorderRows picks the page's rows. all = every match on one page. With a
// plugin annotator (generic: e.g. an equivalent product still in stock) every
// match is annotated, and hideCovered drops the covered ones and pulls in the
// products the plugins name — BEFORE paging, so the pager counts what shows.
// notes is nil when no plugin annotates. Annotators are best-effort.
func (h *handlers) reorderRows(ctx context.Context, q products.ListQuery, all, hideCovered bool) (rows []products.Product, total int, notes map[int64]plugin.ReorderNote, hidden int, err error) {
	anns := plugin.ProductReorderAnnotators()
	if len(anns) == 0 {
		if all {
			rows, err = h.walkLow(ctx, q)
			return rows, len(rows), nil, 0, err
		}
		rows, total, err = h.t.products.List(ctx, q)
		return rows, total, nil, 0, err
	}
	if hideCovered {
		for _, a := range anns {
			if a.LowMembers == nil {
				continue
			}
			if ids, err := a.LowMembers(ctx); err == nil {
				q.AlsoIDs = append(q.AlsoIDs, ids...)
			}
		}
	}
	every, err := h.walkLow(ctx, q)
	if err != nil {
		return nil, 0, nil, 0, err
	}
	ids := make([]int64, 0, len(every))
	for _, p := range every {
		ids = append(ids, p.ID)
	}
	notes = map[int64]plugin.ReorderNote{}
	for _, a := range anns {
		m, err := a.Batch(ctx, ids)
		if err != nil {
			continue
		}
		for id, n := range m {
			cur := notes[id]
			if cur.Note != "" && n.Note != "" {
				n.Note = cur.Note + " · " + n.Note
			}
			notes[id] = plugin.ReorderNote{Covered: cur.Covered || n.Covered, Note: n.Note, Suggest: decimal.Max(cur.Suggest, n.Suggest)}
		}
	}
	for _, p := range every {
		if hideCovered && notes[p.ID].Covered {
			hidden++
			continue
		}
		rows = append(rows, p)
	}
	total = len(rows)
	if !all {
		from := min((q.Page-1)*q.Limit, total)
		rows = rows[from:min(from+q.Limit, total)]
	}
	return rows, total, notes, hidden, nil
}

// AdminReorderBuild turns the picked lines into one draft purchase order per
// supplier; the SPA then opens their print.
func (h *handlers) AdminReorderBuild(c echo.Context) error {
	var req ReorderRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := purchases.ReorderPOInput{Lines: make([]purchases.ReorderLineInput, 0, len(req.Lines))}
	for _, l := range req.Lines {
		in.Lines = append(in.Lines, purchases.ReorderLineInput{ProductID: l.ProductID, SupplierID: l.SupplierID, Quantity: strings.TrimSpace(l.Quantity), CostPrice: strings.TrimSpace(l.CostPrice)})
	}
	ids, err := purchases.NewService(h.d.DB).CreateDraftsFromReorder(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase", "", "built "+strconv.Itoa(len(ids))+" purchase orders from reorder")
	return c.JSON(http.StatusCreated, IDsResult{IDs: ids})
}
