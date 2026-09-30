package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const movementPageSize = 30

// movementTypes are the stock_movement_type enum values a filter may name.
var movementTypes = map[string]bool{"purchase": true, "sale": true, "adjust": true, "return": true, "damage": true, "conversion": true,
	"purchase_return": true, "warranty_replacement": true, "recovery": true, "own_use": true, "staff": true}

func (h *handlers) registerAdminStock(g *echo.Group) {
	g.GET("/stock/movements", h.AdminStockMovements)
	g.POST("/stock/adjust", h.AdminStockAdjust)
	g.GET("/stock/lots/:id", h.AdminProductLots)
	g.POST("/stock/batches/:id/price", h.AdminBatchPrice)
	g.GET("/staff-options", h.AdminStaffOptions)
}

func movementRow(m stock.Movement) MovementRow {
	return MovementRow{ID: m.ID, ProductID: m.ProductID, ProductName: m.ProductName, Type: m.Type, Quantity: m.Quantity.String(),
		Cost: dec2(m.Cost), Note: strOr(m.Note), UserName: m.UserName, CreatedAt: m.CreatedAt.Format(time.RFC3339)}
}

// AdminStockMovements is the stock audit trail: every in/out, who and why,
// newest first. Filters: ?product= ?type= ?user= and a period (listRange).
func (h *handlers) AdminStockMovements(c echo.Context) error {
	f := stock.MovementFilter{Type: c.QueryParam("type")}
	if f.Type != "" && !movementTypes[f.Type] {
		return apperr.Validation("unknown movement type")
	}
	if id, err := strconv.ParseInt(c.QueryParam("product"), 10, 64); err == nil && id > 0 {
		f.ProductID = &id
	}
	if id, err := strconv.ParseInt(c.QueryParam("user"), 10, 64); err == nil && id > 0 {
		f.UserID = &id
	}
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	f.From, f.To = from, to
	if c.QueryParam("format") != "" { // download every match (karots-pos "Download CSV"), not a page
		rows, _, err := h.t.stock.FindMovements(c.Request().Context(), f)
		if err != nil {
			return err
		}
		out := make([][]string, 0, len(rows))
		for _, m := range rows {
			out = append(out, []string{m.CreatedAt.Format("2006-01-02 15:04"), m.ProductName, m.Type, m.Quantity.String(), dec2(m.Cost), m.UserName, strOr(m.Note)})
		}
		return writeSheet(c, "stock_movements_"+time.Now().Format("2006-01-02"), []string{"When", "Product", "Type", "Qty", "Cost", "By", "Note"}, out)
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	page = max(page, 1)
	f.Limit, f.Offset = movementPageSize, (page-1)*movementPageSize
	rows, total, err := h.t.stock.FindMovements(c.Request().Context(), f)
	if err != nil {
		return err
	}
	out := MovementList{Items: make([]MovementRow, 0, len(rows)), Total: total, Page: page, PageSize: movementPageSize}
	for _, m := range rows {
		out.Items = append(out.Items, movementRow(m))
	}
	return c.JSON(http.StatusOK, out)
}

// AdminStockAdjust sets an absolute count (lot-aware on a reduction), booked as
// an 'adjust' movement with its value, so shrinkage reaches the P&L.
func (h *handlers) AdminStockAdjust(c echo.Context) error {
	var req AdjustRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := stock.AdjustInput{ProductID: req.ProductID, NewQuantity: strings.TrimSpace(req.NewQuantity), Note: strings.TrimSpace(req.Note), BatchID: req.BatchID}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.t.stock.Adjust(c.Request().Context(), in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock", strconv.FormatInt(req.ProductID, 10), "adjusted to "+in.NewQuantity)
	return c.NoContent(http.StatusNoContent)
}

// AdminProductLots is a product's live lots with cost (admin only; the till's
// lot list has no cost).
func (h *handlers) AdminProductLots(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	rows, err := h.t.stock.Batches(ctx, id)
	if err != nil {
		return err
	}
	out := ProductLots{ProductID: p.ID, Name: p.Name, UnitAbbr: p.UnitAbbr, SellingPrice: dec2(p.SellingPrice), StockQty: p.StockQty.String(), Lots: make([]LotRow, 0, len(rows))}
	for _, b := range rows {
		out.Lots = append(out.Lots, lotRow(b))
	}
	return c.JSON(http.StatusOK, out)
}

func lotRow(b stock.Batch) LotRow {
	r := LotRow{ID: b.ID, BatchNo: strOr(b.BatchNo), QtyReceived: b.QtyReceived.String(), QtyRemaining: b.QtyRemaining.String(),
		CostPrice: dec2(b.CostPrice), Source: b.Source, CreatedAt: b.CreatedAt.Format(time.RFC3339), FromDelivery: b.PurchaseItemID != nil}
	if b.ExpiryDate != nil {
		r.ExpiryDate = b.ExpiryDate.Format("2006-01-02")
	}
	if b.SellingPrice.IsPositive() {
		r.SellingPrice = dec2(b.SellingPrice)
	}
	return r
}

// AdminBatchPrice re-prices one lot. Blank = 0 = follow the product again (the
// sentinel every lot starts with).
func (h *handlers) AdminBatchPrice(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req LotPriceRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	price := decimal.Zero
	if s := strings.TrimSpace(req.SellingPrice); s != "" {
		if price, err = money.Parse(s); err != nil || price.IsNegative() {
			return apperr.Validation("enter a valid price, or leave it blank to follow the product")
		}
	}
	if err := h.t.stock.SetBatchPrice(c.Request().Context(), id, price); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock_batch", strconv.FormatInt(id, 10), "batch price set to "+price.String())
	return c.NoContent(http.StatusNoContent)
}

// AdminStaffOptions feeds "By" filters: every user but the hidden system one
// (a since-disabled cashier stays listed so their history stays filterable).
func (h *handlers) AdminStaffOptions(c echo.Context) error {
	users, err := h.d.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]Option, 0, len(users))
	for _, u := range users {
		if !u.IsSystem {
			out = append(out, Option{ID: u.ID, Name: u.Name})
		}
	}
	return c.JSON(http.StatusOK, out)
}
