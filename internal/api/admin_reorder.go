package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const reorderPageSize = 30

func (h *handlers) registerAdminReorder(g *echo.Group) {
	g.GET("/reorder", h.AdminReorder) // builds with POST /purchases/draft (admin_purchases.go)
}

// AdminReorder is the low-stock worklist with a demand hint per product
// (karots-pos low-stock PO builder + reorderDemand): search, category (with its
// sub-categories), preferred supplier; ?all=1 every match on one page.
func (h *handlers) AdminReorder(c echo.Context) error {
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	q := products.ListQuery{Search: strings.TrimSpace(c.QueryParam("q")), LowStock: true, Page: page, Limit: reorderPageSize}
	if id, err := strconv.ParseInt(c.QueryParam("category"), 10, 64); err == nil && id > 0 {
		q.CategoryID = &id
	}
	if id, err := strconv.ParseInt(c.QueryParam("supplier"), 10, 64); err == nil && id > 0 {
		q.PreferredSupplierID = &id
	}
	ctx := c.Request().Context()
	all := c.QueryParam("all") == "1"
	rows, total, notes, hidden, err := h.reorderRows(ctx, q, all, c.QueryParam("hideCovered") == "1")
	if err != nil {
		return err
	}
	if all {
		page = 1
	}
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.ID)
	}
	const trailing, lead = 90, 14
	rep := reports.NewService(h.d.DB)
	now := time.Now()
	sold := func(from, to time.Time) map[int64]decimal.Decimal {
		m, err := rep.ProductQtySold(ctx, ids, from, to)
		if err != nil {
			return map[int64]decimal.Decimal{}
		}
		return m
	}
	s90, week, month := sold(now.AddDate(0, 0, -trailing), now), sold(now.AddDate(0, 0, -7), now), sold(now.AddDate(0, 0, -30), now)
	lyTo := now.AddDate(-1, 0, 0)
	ly := sold(lyTo.AddDate(0, 0, -trailing), lyTo)
	txt := func(m map[int64]decimal.Decimal, id int64) string {
		if v, ok := m[id]; ok && v.IsPositive() {
			return money.Display(v)
		}
		return ""
	}
	out := ReorderList{Items: make([]ReorderRow, 0, len(rows)), Total: total, Page: page, PageSize: reorderPageSize,
		Annotated: notes != nil, Hidden: hidden}
	for _, p := range rows {
		r := ReorderRow{ProductID: p.ID, Name: p.Name, UnitAbbr: p.UnitAbbr, OnHand: p.StockQty.String(), ReorderLevel: p.ReorderLevel, Cost: dec2(p.CostPrice),
			SupplierName: strOr(p.PreferredSupplierName), SoldWeek: txt(week, p.ID), SoldMonth: txt(month, p.ID), SoldLastYear: txt(ly, p.ID)}
		if p.PreferredSupplierID != nil {
			r.SupplierID = *p.PreferredSupplierID
		}
		r.Note, r.Covered = notes[p.ID].Note, notes[p.ID].Covered
		need := decimal.NewFromInt(int64(2 * p.ReorderLevel)).Sub(p.StockQty)
		if v := s90[p.ID]; v.IsPositive() {
			need = v.Div(decimal.NewFromInt(trailing)).Mul(decimal.NewFromInt(lead)).Sub(p.StockQty).Ceil()
			r.FromSales = true
		}
		r.Suggested = decimal.Max(need, notes[p.ID].Suggest, decimal.Zero).String()
		out.Items = append(out.Items, r)
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
// supplier (karots-pos PurchaseDraftCreate); the SPA then opens their print.
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
