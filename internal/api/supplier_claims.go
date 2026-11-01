package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/recovery"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// Settling with a supplier where their delivery is received (admin purchases
// and the counter's Suppliers tab): their expired and close-to-expiry batches
// and the write-offs still waiting on them. Mounted on /admin and the counter's
// supplier group.
func registerSupplierClaims(g *echo.Group, h *handlers) {
	g.GET("/suppliers/:id/claims", h.SupplierClaims)
	g.POST("/suppliers/:id/claims/lots/:lot/give-back", h.SupplierGiveBack)
	g.POST("/suppliers/:id/claims/lots/:lot/told", h.SupplierTold)
}

func claimLots(rows []stock.Batch) []ClaimLot {
	out := make([]ClaimLot, 0, len(rows))
	for _, b := range rows {
		l := ClaimLot{LotID: b.ID, ProductID: b.ProductID, Product: b.ProductName, Unit: b.UnitAbbr, Qty: b.QtyRemaining.String(),
			Expiry: b.ExpiryDate.Format(time.DateOnly), Cost: dec2(b.CostPrice), Supplier: b.SupplierName, WarnDays: b.WarnDays}
		if b.SupplierToldOn != nil {
			l.ToldOn = b.SupplierToldOn.Format(time.DateOnly)
		}
		out = append(out, l)
	}
	return out
}

func (h *handlers) SupplierClaims(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	expired, expiring, waiting, err := recovery.NewService(h.d.DB).Claims(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := SupplierClaims{Expired: claimLots(expired), Expiring: claimLots(expiring), Waiting: make([]LossRow, 0, len(waiting))}
	for _, l := range waiting {
		out.Waiting = append(out.Waiting, LossRow{Kind: l.Kind, SourceID: l.MovementID, ID: l.MovementID, ProductID: l.ProductID, Product: l.Product,
			Qty: l.Quantity.String(), Loss: dec2(l.LossValue), Note: strOr(l.Note), By: l.By, At: l.At.Format(time.RFC3339), GivenBack: l.GivenBack})
	}
	return c.JSON(http.StatusOK, out)
}

// supplierLot is a live lot of the supplier in the path; another's is not found.
func (h *handlers) supplierLot(c echo.Context) (*stock.Batch, error) {
	id, err := pathID(c)
	if err != nil {
		return nil, err
	}
	lotID, err := strconv.ParseInt(c.Param("lot"), 10, 64)
	if err != nil {
		return nil, apperr.BadRequest("invalid batch")
	}
	ctx := c.Request().Context()
	var productID int64
	if err := h.d.DB.GetContext(ctx, &productID, `SELECT product_id FROM stock_batches WHERE id = $1`, lotID); err != nil {
		return nil, apperr.NotFound("batch")
	}
	lots, err := h.t.stock.Batches(ctx, productID)
	if err != nil {
		return nil, err
	}
	for i := range lots {
		if lots[i].ID == lotID {
			if lots[i].SupplierID == nil || *lots[i].SupplierID != id {
				return nil, apperr.NotFound("batch")
			}
			return &lots[i], nil
		}
	}
	return nil, apperr.Conflict("that batch has already gone")
}

// SupplierGiveBack hands a counted lot to its supplier; the answer's loss (0 when
// nothing was counted) is what the Recover form then settles.
func (h *handlers) SupplierGiveBack(c echo.Context) error {
	var req GiveBackRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	b, err := h.supplierLot(c)
	if err != nil {
		return err
	}
	loss, err := h.t.stock.GiveBack(c.Request().Context(), stock.GiveBackInput{BatchID: b.ID, Counted: req.Counted, SupplierName: b.SupplierName}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock", strconv.FormatInt(b.ProductID, 10), "gave back "+strings.TrimSpace(req.Counted)+" to "+b.SupplierName)
	return c.JSON(http.StatusOK, ConsumeResult{LossID: loss})
}

// SupplierTold records that the supplier was told about the lot, today.
func (h *handlers) SupplierTold(c echo.Context) error {
	b, err := h.supplierLot(c)
	if err != nil {
		return err
	}
	if err := h.t.stock.MarkSupplierTold(c.Request().Context(), b.ID); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock", strconv.FormatInt(b.ProductID, 10), "told "+b.SupplierName+" about a batch")
	return c.NoContent(http.StatusNoContent)
}
