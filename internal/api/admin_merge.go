package api

import (
	"fmt"
	"net/http"
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"

	"github.com/labstack/echo/v4"
)

// mergeIDs reads the duplicate (:id) and the product kept (:into).
func mergeIDs(c echo.Context) (int64, int64, error) {
	from, err := pathID(c)
	if err != nil {
		return 0, 0, err
	}
	into, err := strconv.ParseInt(c.Param("into"), 10, 64)
	if err != nil || into <= 0 {
		return 0, 0, apperr.BadRequest("invalid product to keep")
	}
	return from, into, nil
}

// AdminMergePreview is the merge confirm screen: what would move, or why it can't.
func (h *handlers) AdminMergePreview(c echo.Context) error {
	from, into, err := mergeIDs(c)
	if err != nil {
		return err
	}
	pv, err := h.t.products.MergePreview(c.Request().Context(), from, into)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, MergePreview{FromName: pv.FromName, IntoName: pv.IntoName, Barcodes: pv.Barcodes,
		Stock: pv.Stock.String(), Lots: pv.Lots, Sales: pv.Sales, Purchases: pv.Purchases, PriceClash: pv.PriceClash,
		FromPrice: dec2(pv.FromPrice), IntoPrice: dec2(pv.IntoPrice), Refusal: pv.Refusal})
}

// AdminMerge folds :id into :into (not undoable; the screen says so).
func (h *handlers) AdminMerge(c echo.Context) error {
	from, into, err := mergeIDs(c)
	if err != nil {
		return err
	}
	var req MergeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	pv, err := h.t.products.MergePreview(ctx, from, into)
	if err != nil {
		return err
	}
	if err := h.t.products.Merge(ctx, from, into, req.LotPrice); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(into, 10),
		fmt.Sprintf("merged %s into %s (%s in stock, %d sales)", pv.FromName, pv.IntoName, pv.Stock.String(), pv.Sales))
	return c.NoContent(http.StatusNoContent)
}
