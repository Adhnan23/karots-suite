package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/conversions"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// AdminProductLoose makes a packed product's loose version: product,
// conversion and (optionally) the first run in one transaction. Plugin extras
// are validated before and saved after, as on a plain create.
func (h *handlers) AdminProductLoose(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req LooseVersionRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	now := decimal.Zero
	if s := strings.TrimSpace(req.ConvertNow); s != "" {
		if now, err = money.Parse(s); err != nil || now.IsNegative() {
			return apperr.Validation("convert now is not a valid amount")
		}
	}
	ctx := c.Request().Context()
	src, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	extras := ProductRequest{Extra: req.Extra}
	x := plugin.ExtraCtx{CategoryID: src.CategoryID, IsNew: true}
	if err := checkExtras(ctx, extras, x); err != nil {
		return err
	}
	pid, cid, err := conversions.MakeLoose(ctx, h.d.DB, conversions.LooseInput{
		SourceID: id, Name: req.Name, UnitID: req.UnitID, Ratio: req.Ratio, SellingPrice: req.SellingPrice,
		Barcode: req.Barcode, ConvertNow: now, UserID: middleware.CurrentUserID(c),
	})
	if err != nil {
		return err
	}
	x.ProductID = pid
	if err := saveExtras(ctx, extras, x); err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(pid, 10),
		"made loose version "+strings.TrimSpace(req.Name)+" of "+src.Name)
	return c.JSON(http.StatusCreated, LooseVersionResult{ProductID: pid, ConversionID: cid})
}
