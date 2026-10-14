package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
)

const stockTakePageSize = 50

// The count sheet layout, and the headers an uploaded sheet may use instead.
var stockTakeColumns = []string{"barcode", "name", "current_qty", "counted_qty", "cost", "selling_price", "wholesale_price", "batch", "expiry"}
var stockTakeSynonyms = map[string]string{
	"code": "barcode", "sku": "barcode", "product": "name", "item": "name",
	"on hand": "current_qty", "current": "current_qty", "system qty": "current_qty",
	"count": "counted_qty", "counted": "counted_qty", "qty": "counted_qty", "quantity": "counted_qty", "physical": "counted_qty", "actual": "counted_qty",
	"cost price": "cost", "cost_price": "cost", "buying price": "cost",
	"selling": "selling_price", "selling price": "selling_price", "sell price": "selling_price", "retail": "selling_price", "retail price": "selling_price", "price": "selling_price",
	"wholesale": "wholesale_price", "wholesale price": "wholesale_price", "trade price": "wholesale_price",
	"lot": "batch", "batch id": "batch", "lot id": "batch", "batch no": "batch", "expiry date": "expiry",
}

func (h *handlers) registerAdminStockTake(g *echo.Group) {
	st := g.Group("/stock/take", h.requireStockTake)
	st.GET("", h.AdminStockTake)
	st.POST("", h.AdminStockTakeApply)
	st.GET("/sheet", h.AdminStockTakeSheet)
	st.POST("/import", h.AdminStockTakeImport)
}

// requireStockTake 403s while Settings → stock-take & intake is off (it can
// rewrite counts and prices). Fails open on a settings read error, as in karots-pos.
func (h *handlers) requireStockTake(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if st, err := h.d.Settings.Get(c.Request().Context()); err == nil && st != nil && !st.StockTakeEnabled {
			return apperr.Forbidden("Stock-take is turned off in Settings")
		}
		return next(c)
	}
}

// AdminStockTake is a page of products to count: search, category, supplier.
func (h *handlers) AdminStockTake(c echo.Context) error {
	ctx := c.Request().Context()
	page, _ := strconv.Atoi(c.QueryParam("page"))
	q := products.ListQuery{Search: strings.TrimSpace(c.QueryParam("q")), Page: page, Limit: stockTakePageSize}
	if id, err := strconv.ParseInt(c.QueryParam("category"), 10, 64); err == nil && id > 0 {
		q.CategoryID = &id
	}
	if id, err := strconv.ParseInt(c.QueryParam("supplier"), 10, 64); err == nil && id > 0 {
		q.PreferredSupplierID = &id
	}
	rows, total, err := h.t.products.List(ctx, q)
	if err != nil {
		return err
	}
	multi, err := h.t.stock.MultiPriceProducts(ctx)
	if err != nil {
		return err
	}
	ids := make([]int64, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
	}
	lots, err := h.t.stock.LiveLotsFor(ctx, ids)
	if err != nil {
		return err
	}
	splits := h.stockSplits(ctx, rows)
	out := StockTakeList{Items: make([]StockTakeRow, 0, len(rows)), Total: total, Page: max(page, 1), PageSize: stockTakePageSize}
	for _, p := range rows {
		_, many := multi[p.ID]
		var lr []LotRow
		if len(lots[p.ID]) >= 2 {
			for _, b := range lots[p.ID] {
				lr = append(lr, lotRow(b))
			}
		}
		out.Items = append(out.Items, StockTakeRow{ID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode), CategoryName: p.CategoryName, UnitAbbr: p.UnitAbbr,
			UnitAllowDecimal: p.UnitAllowDecimal, StockQty: p.StockQty.String(), CostPrice: dec2(p.CostPrice), SellingPrice: dec2(p.SellingPrice),
			WholesalePrice: dec2(p.WholesalePrice), MultiPrice: many, Lots: lr, StockSplit: splits[p.ID]})
	}
	return c.JSON(http.StatusOK, out)
}

// applyCount applies one counted row: cost first (so a new opening lot is valued
// right), then selling/wholesale, then the count as an audited adjustment. A
// blank field leaves that value alone. Reports whether anything changed.
func (h *handlers) applyCount(ctx context.Context, p *products.Product, l CountLine, note string, userID int64) (bool, error) {
	changed := false
	if s := strings.TrimSpace(l.Cost); s != "" {
		cost, err := money.Parse(s)
		if err != nil || cost.IsNegative() {
			return false, apperr.Validation("cost must be a non-negative amount")
		}
		if !cost.Equal(p.CostPrice) {
			if err := h.t.products.SetCost(ctx, p.ID, cost); err != nil {
				return false, err
			}
			changed = true
		}
	}
	sell, whole := p.SellingPrice, p.WholesalePrice
	if s := strings.TrimSpace(l.Selling); s != "" {
		v, err := money.Parse(s)
		if err != nil || v.IsNegative() {
			return false, apperr.Validation("selling price must be a non-negative amount")
		}
		sell = v
	}
	if s := strings.TrimSpace(l.Wholesale); s != "" {
		v, err := money.Parse(s)
		if err != nil || v.IsNegative() {
			return false, apperr.Validation("wholesale price must be a non-negative amount")
		}
		whole = v
	}
	if !sell.Equal(p.SellingPrice) || !whole.Equal(p.WholesalePrice) {
		if err := h.t.products.SetPrices(ctx, p.ID, sell, whole); err != nil {
			return false, err
		}
		changed = true
	}
	if len(l.Batches) > 0 || len(l.NewBatches) > 0 {
		c, err := h.applyLotCounts(ctx, l, note, userID)
		return changed || c, err
	}
	if s := strings.TrimSpace(l.Counted); s != "" {
		target, err := money.Parse(s)
		if err != nil || target.IsNegative() {
			return false, apperr.Validation("the count must be a non-negative number")
		}
		if cur, err := h.t.stock.Quantity(ctx, p.ID); err != nil || !cur.Equal(target) {
			if err := h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: p.ID, NewQuantity: s, Note: note}, userID); err != nil {
				return false, err
			}
			changed = true
		}
	}
	return changed, nil
}

// AdminStockTakeApply saves the counted rows. Each row stands alone: a bad row
// is reported by product, the rest still save.
func (h *handlers) AdminStockTakeApply(c echo.Context) error {
	var req StockTakeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	saved := 0
	for _, l := range req.Lines {
		p, err := h.t.products.Get(ctx, l.ProductID)
		if err != nil {
			return err
		}
		changed, err := h.applyCount(ctx, p, l, "stock-take", uid)
		if err != nil {
			return apperr.Validation(p.Name + ": " + importErrMsg(err))
		}
		if changed {
			saved++
		}
	}
	h.audit(c, audit.ActionUpdate, "stock", "", fmt.Sprintf("stock-take: %d products updated", saved))
	return c.JSON(http.StatusOK, StockTakeResult{Saved: saved})
}

// AdminStockTakeSheet downloads the count sheet: every active product with its
// on-hand, cost and prices, and a blank counted_qty column — one row per batch
// for a product with 2+ batches (the batch column says which).
func (h *handlers) AdminStockTakeSheet(c echo.Context) error {
	rows, err := h.t.products.ListAll(c.Request().Context(), false)
	if err != nil {
		return err
	}
	ids := make([]int64, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
	}
	lots, err := h.t.stock.LiveLotsFor(c.Request().Context(), ids)
	if err != nil {
		return err
	}
	out := make([][]string, 0, len(rows))
	for _, p := range rows {
		if len(lots[p.ID]) >= 2 {
			out = append(out, lotSheetRows(p, lots[p.ID])...)
			continue
		}
		out = append(out, []string{strOr(p.Barcode), p.Name, p.StockQty.String(), "", dec2(p.CostPrice), dec2(p.SellingPrice), dec2(p.WholesalePrice), "", ""})
	}
	return writeSheet(c, "stock-count-sheet", stockTakeColumns, out)
}

// AdminStockTakeImport applies a filled count sheet, matched by barcode. Blank
// counted_qty rows are left alone; unchanged rows are tallied, not rewritten.
func (h *handlers) AdminStockTakeImport(c echo.Context) error {
	col, recs, err := readImportSheet(c, stockTakeSynonyms, "barcode")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	sum := ImportSummary{Errors: []ImportRowError{}, Notes: []string{}}
	fail := func(line int, msg string) {
		sum.Errors = append(sum.Errors, ImportRowError{Line: line, Message: msg})
		sum.Skipped++
	}
	for i, rec := range recs {
		line := i + 2 // 1-based, after the header
		if i >= maxImportRows {
			sum.Notes = append(sum.Notes, fmt.Sprintf("stopped at %d rows (limit)", maxImportRows))
			break
		}
		get := cellGetter(col, rec)
		barcode, counted := get("barcode"), get("counted_qty")
		if counted == "" {
			continue // not counted (or a blank trailing row)
		}
		if barcode == "" {
			fail(line, "a barcode is needed to match the product")
			continue
		}
		p, err := h.t.products.GetByBarcode(ctx, barcode)
		if err != nil {
			fail(line, "no product with barcode "+barcode)
			continue
		}
		var changed bool
		if batch := get("batch"); batch != "" {
			changed, err = h.importLotRow(ctx, p, batch, counted, get("cost"), get("selling_price"), uid)
		} else {
			changed, err = h.applyCount(ctx, p, CountLine{ProductID: p.ID, Counted: counted, Cost: get("cost"), Selling: get("selling_price"), Wholesale: get("wholesale_price")}, "stock-take sheet", uid)
		}
		switch {
		case err != nil:
			fail(line, importErrMsg(err))
		case changed:
			sum.Updated++
		default:
			sum.Unchanged++
		}
	}
	h.audit(c, audit.ActionUpdate, "stock", "", fmt.Sprintf("stock-take sheet: %d updated, %d unchanged, %d skipped", sum.Updated, sum.Unchanged, sum.Skipped))
	return c.JSON(http.StatusOK, sum)
}
