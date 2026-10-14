package api

import (
	"context"
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

// foldSource is the active card → this-product conversion, if any (the first).
func (h *handlers) foldSource(ctx context.Context, productID int64) *FoldSource {
	var cv struct {
		ID      int64           `db:"id"`
		Name    string          `db:"name"`
		Ratio   decimal.Decimal `db:"ratio"`
		Selling decimal.Decimal `db:"selling_price"`
		Stock   decimal.Decimal `db:"stock"`
		Unit    string          `db:"unit"`
	}
	err := h.d.DB.GetContext(ctx, &cv, `
		SELECT cv.id, p.name, cv.ratio, p.selling_price, COALESCE(s.quantity, 0) AS stock, u.abbreviation AS unit
		FROM product_conversions cv
		JOIN products p ON p.id = cv.from_product_id
		JOIN units u ON u.id = p.unit_id
		LEFT JOIN stock s ON s.product_id = p.id
		WHERE cv.to_product_id = $1 AND cv.is_active AND p.is_active
		ORDER BY cv.id LIMIT 1`, productID)
	if err != nil {
		return nil
	}
	return &FoldSource{ConversionID: cv.ID, Name: cv.Name, Ratio: cv.Ratio.String(), SellingPrice: dec2(cv.Selling), Stock: cv.Stock.String(), UnitAbbr: cv.Unit}
}

// AdminProductFold folds a card + loose twin into this (loose) product with a pack.
func (h *handlers) AdminProductFold(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req FoldRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	src := h.foldSource(ctx, id)
	if src == nil || src.ConversionID != req.ConversionID {
		return apperr.Validation("that card isn't cut into this product")
	}
	if _, err := conversions.Fold(ctx, h.d.DB, req.ConversionID, req.Name, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	if n := strings.TrimSpace(req.Rename); n != "" {
		if err := h.t.products.Rename(ctx, id, n); err != nil {
			return err
		}
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "folded "+src.Name+" into a pack ("+strings.TrimSpace(req.Name)+" ×"+src.Ratio+")")
	return c.NoContent(http.StatusNoContent)
}
