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

// Stock intake: add or restock one item at a
// time, set the count and print its labels. Behind the stock-take toggle.
func (h *handlers) registerAdminIntake(g *echo.Group) {
	in := g.Group("/inventory/intake", h.requireStockTake)
	in.POST("/create", h.AdminIntakeCreate)
	in.POST("/restock", h.AdminIntakeRestock)
	in.POST("/undo", h.AdminIntakeUndo)
}

func (h *handlers) intakeItem(ctx context.Context, id int64, qty, kind string, batchID int64) (IntakeItem, error) {
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return IntakeItem{}, err
	}
	return IntakeItem{ProductID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode), SellingPrice: dec2(p.SellingPrice), Qty: qty, Kind: kind,
		StockQty: p.StockQty.String(), UnitAbbr: p.UnitAbbr, BatchID: batchID}, nil
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
	var bid int64
	if n.IsPositive() {
		// The opening batch follows the product's price (stamping it would freeze it).
		if bid, err = h.t.stock.AddStock(ctx, stock.AddStockInput{ProductID: p.ID, Qty: n.String(), Expiry: req.Expiry, Note: "stock intake"}, middleware.CurrentUserID(c)); err != nil {
			return err
		}
	}
	item, err := h.intakeItem(ctx, p.ID, n.String(), "create", bid)
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
	// A top-up keeps that batch's own cost / price / expiry; a new price is
	// stamped only on a new batch.
	bid, err := h.t.stock.AddStock(ctx, stock.AddStockInput{ProductID: p.ID, BatchID: req.BatchID, Qty: add.String(),
		Price: strings.TrimSpace(req.SellingPrice), Expiry: req.Expiry, Note: "stock intake restock"}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	if req.Scanned != "" {
		h.t.products.BarcodeScanned(ctx, req.Scanned)
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(p.ID, 10), "intake restock +"+add.String())
	item, err := h.intakeItem(ctx, p.ID, add.String(), "restock", bid)
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
		// Off the batch the row went into, unless some of it has sold since: then oldest first.
		bid := req.BatchID
		if bid > 0 && !h.lotHolds(ctx, p.ID, bid, p.StockQty.Sub(left)) {
			bid = 0
		}
		if err := h.t.stock.Adjust(ctx, stock.AdjustInput{ProductID: p.ID, NewQuantity: left.String(), Note: "stock intake undo", BatchID: bid}, middleware.CurrentUserID(c)); err != nil {
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

// lotHolds reports whether a live batch of the product still holds qty.
func (h *handlers) lotHolds(ctx context.Context, productID, batchID int64, qty decimal.Decimal) bool {
	lots, err := h.t.stock.Batches(ctx, productID)
	if err != nil {
		return false
	}
	for _, b := range lots {
		if b.ID == batchID {
			return !b.QtyRemaining.LessThan(qty)
		}
	}
	return false
}
