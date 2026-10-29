package api

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminBatches(g *echo.Group) {
	g.GET("/stock/batches", h.AdminBatches)
	g.POST("/stock/expired/write-off-all", h.AdminWriteOffExpired)
	g.POST("/stock/damage", h.AdminStockConsume)
}

// AdminBatches is every live lot, grouped by product: expired first, then
// expiring within 30 days, then the rest by name (karots-pos Batches page and
// Batches / Expiry report); ?format= downloads every lot.
func (h *handlers) AdminBatches(c echo.Context) error {
	rows, err := h.t.stock.AllBatches(c.Request().Context())
	if err != nil {
		return err
	}
	if wantsSheet(c) {
		return batchesSheet(c, rows)
	}
	now := time.Now()
	soon := now.AddDate(0, 0, 30)
	var total, expVal, soonVal decimal.Decimal
	out := BatchesView{Groups: []BatchGroup{}}
	idx := map[int64]int{}
	onHand, value := map[int64]decimal.Decimal{}, map[int64]decimal.Decimal{}
	for _, b := range rows {
		val := b.QtyRemaining.Mul(b.CostPrice)
		total = total.Add(val)
		expired := b.ExpiryDate != nil && b.ExpiryDate.Before(now)
		expiring := !expired && b.ExpiryDate != nil && b.ExpiryDate.Before(soon)
		switch {
		case b.ExpiryDate == nil:
			out.NoExpiry++
		case expired:
			out.Expired++
			expVal = expVal.Add(val)
		case expiring:
			out.ExpiringSoon++
			soonVal = soonVal.Add(val)
		}
		gi, ok := idx[b.ProductID]
		if !ok {
			out.Groups = append(out.Groups, BatchGroup{ProductID: b.ProductID, Name: b.ProductName, UnitAbbr: b.UnitAbbr, ProductPrice: dec2(b.ProductPrice), Lots: []LotRow{}})
			gi = len(out.Groups) - 1
			idx[b.ProductID] = gi
		}
		g := &out.Groups[gi]
		g.Lots = append(g.Lots, lotRow(b))
		onHand[b.ProductID] = onHand[b.ProductID].Add(b.QtyRemaining)
		value[b.ProductID] = value[b.ProductID].Add(val)
		if expired {
			g.Expired++
		}
		if expiring {
			g.ExpiringSoon++
		}
	}
	for i := range out.Groups {
		g := &out.Groups[i]
		g.OnHand, g.Value = onHand[g.ProductID].String(), dec2(value[g.ProductID])
	}
	rank := func(g BatchGroup) int {
		switch {
		case g.Expired > 0:
			return 0
		case g.ExpiringSoon > 0:
			return 1
		}
		return 2
	}
	slices.SortStableFunc(out.Groups, func(x, y BatchGroup) int {
		if rx, ry := rank(x), rank(y); rx != ry {
			return rx - ry
		}
		return strings.Compare(x.Name, y.Name)
	})
	out.TotalValue, out.ExpiredValue, out.ExpiringValue = dec2(total), dec2(expVal), dec2(soonVal)
	return c.JSON(http.StatusOK, out)
}

// AdminWriteOffExpired writes off every expired lot as damage, one lot per
// transaction (each write-off stands on its own; a failure stops the run and
// reports it, the lots already done stay done).
func (h *handlers) AdminWriteOffExpired(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.t.stock.Expiring(ctx, 0)
	if err != nil {
		return err
	}
	now := time.Now()
	n := 0
	for _, b := range rows {
		if b.ExpiryDate == nil || !b.ExpiryDate.Before(now) || !b.QtyRemaining.IsPositive() {
			continue
		}
		if err := h.t.stock.Consume(ctx, stock.ConsumeInput{ProductID: b.ProductID, Quantity: b.QtyRemaining.String(), Reason: "damage",
			BatchID: b.ID, Note: "expired " + b.ExpiryDate.Format("2006-01-02")}, middleware.CurrentUserID(c)); err != nil {
			return err
		}
		n++
	}
	if n > 0 {
		h.audit(c, audit.ActionUpdate, "stock", "", "wrote off "+strconv.Itoa(n)+" expired lot(s)")
	}
	return c.JSON(http.StatusOK, WriteOffResult{Lots: n})
}

// AdminStockConsume takes stock out without a sale — damage, shop use or staff —
// from a named lot or oldest first.
func (h *handlers) AdminStockConsume(c echo.Context) error {
	var req ConsumeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := stock.ConsumeInput{ProductID: req.ProductID, Quantity: strings.TrimSpace(req.Quantity), Reason: req.Reason, Note: strings.TrimSpace(req.Note), BatchID: req.BatchID}
	if err := c.Validate(&in); err != nil {
		return err
	}
	id, err := h.t.stock.ConsumeID(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "stock", strconv.FormatInt(req.ProductID, 10), req.Reason+" "+in.Quantity)
	out := ConsumeResult{}
	if req.Reason == "damage" {
		out.LossID = id
	}
	return c.JSON(http.StatusOK, out)
}

// batchesSheet is every live lot as a download (karots-pos Batches report
// CSV). "Own price" says whether the lot carries its own price (and so can be
// picked at the till) or follows the product.
func batchesSheet(c echo.Context, rows []stock.Batch) error {
	out := make([][]string, 0, len(rows))
	for _, b := range rows {
		expiry, own := "", "no"
		if b.ExpiryDate != nil {
			expiry = b.ExpiryDate.Format("2006-01-02")
		}
		if b.SellingPrice.IsPositive() {
			own = "yes"
		}
		out = append(out, []string{b.ProductName, strOr(b.BatchNo), expiry, b.QtyRemaining.String(), b.UnitAbbr, dec2(b.CostPrice),
			dec2(b.EffectivePrice(b.ProductPrice)), own, dec2(b.QtyRemaining.Mul(b.CostPrice))})
	}
	return writeSheet(c, "batches_"+time.Now().Format("2006-01-02"), []string{"Product", "Batch", "Expiry", "Remaining", "Unit", "Cost", "Sells at", "Own price", "Value"}, out)
}
