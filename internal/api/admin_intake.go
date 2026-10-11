package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// Stock intake (karots-pos /admin/inventory/intake): add or restock one item at a
// time, set the count and print its labels. Behind the stock-take toggle.
func (h *handlers) registerAdminIntake(g *echo.Group) {
	in := g.Group("/inventory/intake", h.requireStockTake)
	in.POST("/create", h.AdminIntakeCreate)
	in.POST("/restock", h.AdminIntakeRestock)
	in.POST("/undo", h.AdminIntakeUndo)
}

func (h *handlers) intakeItem(ctx context.Context, id int64, qty, kind string) (IntakeItem, error) {
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return IntakeItem{}, err
	}
	return IntakeItem{ProductID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode), SellingPrice: dec2(p.SellingPrice), Qty: qty, Kind: kind,
		StockQty: p.StockQty.String(), UnitAbbr: p.UnitAbbr}, nil
}

// AdminIntakeCreate adds a new product and its opening count. A name that
// already exists is refused: restock that one instead of making a twin.
func (h *handlers) AdminIntakeCreate(c echo.Context) error {
	var req IntakeCreateRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in, _, err := bindProductReq(c, req.Product)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if existing, err := h.t.products.FindByName(ctx, in.Name); err != nil {
		return err
	} else if existing != nil {
		return apperr.Conflict("“" + existing.Name + "” already exists — search for it to add stock instead of creating a duplicate")
	}
	qty := strings.TrimSpace(req.Quantity)
	var n decimal.Decimal
	if qty != "" {
		if n, err = money.Parse(qty); err != nil || n.IsNegative() {
			return apperr.Validation("the quantity must be a number")
		}
	}
	x := plugin.ExtraCtx{CategoryID: in.CategoryID, IsNew: true}
	if err := checkExtras(ctx, req.Product, x); err != nil {
		return err
	}
	p, err := h.t.products.Create(ctx, in)
	if err != nil {
		return err
	}
	x.ProductID = p.ID
	if err := saveExtras(ctx, req.Product, x); err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), "intake created "+in.Name)
	if n.IsPositive() {
		if err := h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: p.ID, NewQuantity: n.String(), Note: "stock intake"}, middleware.CurrentUserID(c)); err != nil {
			return err
		}
	}
	item, err := h.intakeItem(ctx, p.ID, n.String(), "create")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, item)
}

// AdminIntakeRestock adds units to an existing product, optionally with a new
// cost and prices; a new selling price is stamped on the lot this opens.
func (h *handlers) AdminIntakeRestock(c echo.Context) error {
	var req IntakeRestockRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	add, err := money.Parse(strings.TrimSpace(req.Quantity))
	if err != nil || !add.IsPositive() {
		return apperr.Validation("enter a quantity to add")
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, req.ProductID)
	if err != nil {
		return err
	}
	// Prices first (cost values the lot this opens), through the stock-take path.
	if _, err := h.applyCount(ctx, p, CountLine{ProductID: p.ID, Cost: req.CostPrice, Selling: req.SellingPrice, Wholesale: req.WholesalePrice}, "", 0); err != nil {
		return err
	}
	if err := h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: p.ID, NewQuantity: p.StockQty.Add(add).String(), Note: "stock intake restock",
		SellingPrice: strings.TrimSpace(req.SellingPrice)}, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(p.ID, 10), "intake restock +"+add.String())
	item, err := h.intakeItem(ctx, p.ID, add.String(), "restock")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, item)
}

// AdminIntakeUndo backs out one session row: the units come off (never below
// zero) and a product this session created is disabled, recoverable from
// Products → Show disabled.
func (h *handlers) AdminIntakeUndo(c echo.Context) error {
	var req IntakeUndoRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, req.ProductID)
	if err != nil {
		return err
	}
	if delta, err := money.Parse(strings.TrimSpace(req.Qty)); err == nil && delta.IsPositive() {
		left := decimal.Max(p.StockQty.Sub(delta), decimal.Zero)
		if err := h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: p.ID, NewQuantity: left.String(), Note: "stock intake undo"}, middleware.CurrentUserID(c)); err != nil {
			return err
		}
	}
	if req.Kind == "create" {
		if err := h.t.products.Delete(ctx, p.ID); err != nil {
			return err
		}
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(p.ID, 10), "intake undo ("+req.Kind+")")
	return c.NoContent(http.StatusNoContent)
}
