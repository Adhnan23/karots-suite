package api

import (
	"fmt"
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// AdminAddBatch opens a batch by hand (Batches → + Add batch) and answers the
// product's batches as they now stand.
func (h *handlers) AdminAddBatch(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AddBatchRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	if _, err := h.t.products.Get(ctx, id); err != nil {
		return err
	}
	bid, err := h.t.stock.AddStock(ctx, stock.AddStockInput{ProductID: id, Qty: req.Qty, Cost: req.Cost, Price: req.SellingPrice,
		Expiry: req.Expiry, Note: "batch added"}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "stock_batch", strconv.FormatInt(bid, 10), "batch added: "+req.Qty)
	return h.AdminProductLots(c)
}

// AdminMergeBatches folds batches into the oldest of them (no stock movement:
// the count doesn't change) and answers the product's batches.
func (h *handlers) AdminMergeBatches(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req MergeBatchesRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	kept, err := h.t.stock.MergeBatches(c.Request().Context(), stock.MergeInput{ProductID: id, BatchIDs: req.BatchIDs,
		Cost: req.Cost, Expiry: req.Expiry, Price: req.SellingPrice})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock_batch", strconv.FormatInt(kept, 10),
		fmt.Sprintf("merged %d batches into #%d (cost %s, expiry %q, price %q)", len(req.BatchIDs), kept, req.Cost, req.Expiry, req.SellingPrice))
	return h.AdminProductLots(c)
}
