package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/features/products"
	"karots-suite/internal/features/reorder"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// buildReorderList is the reorder worklist behind Reorder (admin) and the till's
// Low stock tab: ?q, ?category (with sub-categories), ?supplier, ?page, ?all=1,
// ?hideCovered=1, ?looksOff=1. Products made from another (loose from a bag)
// never show: the original is reordered. withCost = false blanks the cost.
func (h *handlers) buildReorderList(c echo.Context, withCost bool) (ReorderList, error) {
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	q := products.ListQuery{Search: strings.TrimSpace(c.QueryParam("q")), LowStock: true, NotDerived: true, Page: page, Limit: reorderPageSize}
	if id, err := strconv.ParseInt(c.QueryParam("category"), 10, 64); err == nil && id > 0 {
		q.CategoryID = &id
	}
	if id, err := strconv.ParseInt(c.QueryParam("supplier"), 10, 64); err == nil && id > 0 {
		q.PreferredSupplierID = &id
	}
	ctx := c.Request().Context()
	all := c.QueryParam("all") == "1"
	var (
		rows   []products.Product
		total  int
		notes  map[int64]plugin.ReorderNote
		hidden int
		err    error
	)
	if c.QueryParam("looksOff") == "1" {
		q.LowStock, q.FixedLearned = false, true
		every, err := h.walkLow(ctx, q)
		if err != nil {
			return ReorderList{}, err
		}
		for _, p := range every {
			if c := calcOf(p); c != nil && reorder.LooksOff(p.ReorderLevel, *c) {
				rows = append(rows, p)
			}
		}
		total = len(rows)
		if !all {
			from := min((page-1)*reorderPageSize, total)
			rows = rows[from:min(from+reorderPageSize, total)]
		}
	} else if rows, total, notes, hidden, err = h.reorderRows(ctx, q, all, c.QueryParam("hideCovered") == "1"); err != nil {
		return ReorderList{}, err
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
		r := ReorderRow{ProductID: p.ID, Name: p.Name, UnitAbbr: p.UnitAbbr, OnHand: p.StockQty.String(), ReorderLevel: p.ReorderLevel,
			SupplierName: strOr(p.PreferredSupplierName), SoldWeek: txt(week, p.ID), SoldMonth: txt(month, p.ID), SoldLastYear: txt(ly, p.ID), Auto: p.ReorderAuto}
		if withCost {
			r.Cost = dec2(p.CostPrice)
		}
		if p.PreferredSupplierID != nil {
			r.SupplierID = *p.PreferredSupplierID
		}
		r.Note, r.Covered = notes[p.ID].Note, notes[p.ID].Covered
		need := decimal.NewFromInt(int64(2 * p.ReorderLevel)).Sub(p.StockQty)
		if calc := calcOf(p); calc != nil && calc.Daily > 0 {
			// Enough to last until the delivery after the next.
			r.Calc, r.LooksOff = calcView(calc), !p.ReorderAuto && reorder.LooksOff(p.ReorderLevel, *calc)
			need = decimal.NewFromInt(int64(p.ReorderLevel)).Add(decimal.NewFromFloat(calc.Daily * float64(calc.RhythmDays))).Sub(p.StockQty).Ceil()
			r.FromSales = true
		} else if calc != nil {
			r.Calc, r.LooksOff = calcView(calc), !p.ReorderAuto && reorder.LooksOff(p.ReorderLevel, *calc)
		} else if v := s90[p.ID]; v.IsPositive() {
			need = v.Div(decimal.NewFromInt(trailing)).Mul(decimal.NewFromInt(lead)).Sub(p.StockQty).Ceil()
			r.FromSales = true
		}
		r.Suggested = decimal.Max(need, notes[p.ID].Suggest, decimal.Zero).String()
		out.Items = append(out.Items, r)
	}
	return out, nil
}

// calcOf decodes a product's learned reorder numbers (nil = the nightly run
// hasn't reached it yet).
func calcOf(p products.Product) *reorder.Calc {
	if len(p.ReorderCalc) == 0 {
		return nil
	}
	var c reorder.Calc
	if json.Unmarshal(p.ReorderCalc, &c) != nil {
		return nil
	}
	return &c
}

func calcView(c *reorder.Calc) *ReorderCalcView {
	if c == nil {
		return nil
	}
	v := ReorderCalcView(*c)
	return &v
}
