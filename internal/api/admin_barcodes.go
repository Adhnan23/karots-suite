package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"

	"github.com/labstack/echo/v4"
)

// barcodesOf is a product's extra codes, never nil (JSON [] not null).
func barcodesOf(p products.Product) []string {
	if len(p.Barcodes) == 0 {
		return []string{}
	}
	return []string(p.Barcodes)
}

func isoOr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// AdminBarcodes lists a product's codes, main first, for the form's Barcodes block.
func (h *handlers) AdminBarcodes(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	rows, err := h.t.products.Barcodes(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := make([]ProductBarcode, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProductBarcode{Barcode: r.Barcode, Main: r.Main, Source: r.Source, AddedAt: isoOr(r.AddedAt), LastScannedAt: isoOr(r.LastScannedAt)})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminBarcodeAdd(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req BarcodeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.t.products.AddBarcode(c.Request().Context(), id, req.Barcode, req.Source); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "added barcode "+req.Barcode)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminBarcodeRemove(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	code := c.Param("code")
	if err := h.t.products.RemoveBarcode(c.Request().Context(), id, code); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "removed barcode "+code)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminBarcodeMain(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	code := c.Param("code")
	if err := h.t.products.MakeMainBarcode(c.Request().Context(), id, code); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "main barcode is now "+code)
	return c.NoContent(http.StatusNoContent)
}
