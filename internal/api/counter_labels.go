package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/printing"
	"karots-suite/internal/tspl"

	"github.com/labstack/echo/v4"
)

// CounterLabels sends a barcode sticker run to the shop's label printer (TSPL;
// blank = the OS default printer, as in karots-pos).
func (h *handlers) CounterLabels(c echo.Context) error {
	ctx := c.Request().Context()
	var req LabelRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	code := strings.TrimSpace(req.Code)
	if code == "" && req.ProductID > 0 {
		p, err := h.t.products.Get(ctx, req.ProductID)
		if err != nil {
			return err
		}
		code = "SKU" + strconv.FormatInt(p.ID, 10)
		if p.Barcode != nil && *p.Barcode != "" {
			code = *p.Barcode
		}
	}
	if code == "" {
		return apperr.Validation("pick a product or enter a barcode")
	}
	w, hgt := labelSize(req.Size, cfg.LabelWidthMM, cfg.LabelHeightMM)
	doc := tspl.Document(tspl.Input{
		Top: strings.TrimSpace(req.Top), Bottom: strings.TrimSpace(req.Bottom),
		NameLines: cfg.LabelNameLines, BottomLines: cfg.LabelBottomLines, NameFont: cfg.LabelNameFont, BottomFont: cfg.LabelBottomFont,
		Code: code, Format: req.Format, Count: min(max(req.Count, 1), 200),
		WidthMM: w, HeightMM: hgt, GapMM: min(max(cfg.LabelGapMM, 0), 20),
	})
	if err := printing.Raw(ctx, cfg.LabelPrinter, doc); err != nil {
		return apperr.Internal("could not print labels", err)
	}
	return c.NoContent(http.StatusNoContent)
}

// labelSize parses a "WxH" preset (mm), falling back to the shop default and
// clamping to what a label printer can take (karots-pos resolveLabelSize).
func labelSize(size string, defW, defH int) (int, int) {
	w, h := defW, defH
	if a, b, ok := strings.Cut(strings.ToLower(strings.TrimSpace(size)), "x"); ok {
		pw, e1 := strconv.Atoi(strings.TrimSpace(a))
		ph, e2 := strconv.Atoi(strings.TrimSpace(b))
		if e1 == nil && e2 == nil && pw > 0 && ph > 0 {
			w, h = pw, ph
		}
	}
	return min(max(w, 10), 200), min(max(h, 10), 200)
}
