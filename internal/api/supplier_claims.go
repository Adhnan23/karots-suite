package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/recovery"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// Settling with a supplier where their delivery is received (admin purchases
// and the counter's Suppliers tab): their expired batches and the write-offs
// still waiting on them. Mounted on /admin and the counter's supplier group.
func registerSupplierClaims(g *echo.Group, h *handlers) {
	g.GET("/suppliers/:id/claims", h.SupplierClaims)
	g.POST("/suppliers/:id/claims/expired/:lot", h.SupplierClaimWriteOff)
}

func (h *handlers) SupplierClaims(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	expired, waiting, err := recovery.NewService(h.d.DB).Claims(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := SupplierClaims{Expired: make([]ClaimLot, 0, len(expired)), Waiting: make([]LossRow, 0, len(waiting))}
	for _, l := range expired {
		out.Expired = append(out.Expired, ClaimLot{LotID: l.LotID, ProductID: l.ProductID, Product: l.Product, Unit: l.Unit, Qty: l.Qty.String(),
			Expiry: l.Expiry.Format(time.DateOnly), Cost: dec2(l.Cost)})
	}
	for _, l := range waiting {
		out.Waiting = append(out.Waiting, LossRow{Kind: l.Kind, SourceID: l.MovementID, ID: l.MovementID, ProductID: l.ProductID, Product: l.Product,
			Qty: l.Quantity.String(), Loss: dec2(l.LossValue), Note: strOr(l.Note), By: l.By, At: l.At.Format(time.RFC3339)})
	}
	return c.JSON(http.StatusOK, out)
}

// SupplierClaimWriteOff writes a whole expired batch off as damage and answers
// the loss, which the Recover form then settles with the supplier.
func (h *handlers) SupplierClaimWriteOff(c echo.Context) error {
	lotID, err := strconv.ParseInt(c.Param("lot"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid batch")
	}
	ctx := c.Request().Context()
	var b stock.Batch
	if err := h.d.DB.GetContext(ctx, &b, `SELECT * FROM stock_batches WHERE id = $1`, lotID); err != nil {
		return apperr.NotFound("batch")
	}
	if !b.QtyRemaining.IsPositive() || b.ExpiryDate == nil || !b.ExpiryDate.Before(time.Now()) {
		return apperr.Validation("that batch has nothing expired left")
	}
	loss, err := h.t.stock.ConsumeID(ctx, stock.ConsumeInput{ProductID: b.ProductID, Quantity: b.QtyRemaining.String(), Reason: "damage",
		BatchID: b.ID, Note: "expired " + b.ExpiryDate.Format(time.DateOnly)}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock", strconv.FormatInt(b.ProductID, 10), "expired batch written off for the supplier")
	return c.JSON(http.StatusOK, ConsumeResult{LossID: loss})
}
