package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/printing"
	"karots-suite/internal/tspl"

	"github.com/labstack/echo/v4"
)

// labelInput turns a label request into the renderer's input using the shop's
// label settings. Code may come back blank (nothing picked yet) — fine for a
// preview, refused by the print.
func (h *handlers) labelInput(c echo.Context) (tspl.Input, string, error) {
	ctx := c.Request().Context()
	var req LabelRequest
	if err := c.Bind(&req); err != nil {
		return tspl.Input{}, "", apperr.BadRequest("invalid request body")
	}
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return tspl.Input{}, "", err
	}
	code := strings.TrimSpace(req.Code)
	if code == "" && req.ProductID > 0 {
		p, err := h.t.products.Get(ctx, req.ProductID)
		if err != nil {
			return tspl.Input{}, "", err
		}
		code = "SKU" + strconv.FormatInt(p.ID, 10)
		if p.Barcode != nil && *p.Barcode != "" {
			code = *p.Barcode
		}
	}
	w, hgt := labelSize(req.Size, cfg.LabelWidthMM, cfg.LabelHeightMM)
	nameLines, bottomLines, nameFont, bottomFont := cfg.LabelNameLines, cfg.LabelBottomLines, cfg.LabelNameFont, cfg.LabelBottomFont
	if l := req.Look; l != nil { // tspl clamps lines and falls back on an unknown font
		nameLines, bottomLines, nameFont, bottomFont = l.FirstLines, l.SecondLines, l.FirstSize, l.SecondSize
	}
	return tspl.Input{
		Top: strings.TrimSpace(req.Top), Bottom: strings.TrimSpace(req.Bottom),
		NameLines: nameLines, BottomLines: bottomLines, NameFont: nameFont, BottomFont: bottomFont,
		Code: code, Format: req.Format, Count: min(max(req.Count, 1), 200),
		WidthMM: w, HeightMM: hgt, GapMM: min(max(cfg.LabelGapMM, 0), 20),
	}, cfg.LabelPrinter, nil
}

// CounterLabels sends a barcode sticker run to the shop's label printer (TSPL;
// blank = the OS default printer).
func (h *handlers) CounterLabels(c echo.Context) error {
	if done, err := h.labelImage(c, 200); done {
		return err
	}
	in, printer, err := h.labelInput(c)
	if err != nil {
		return err
	}
	if in.Code == "" {
		return apperr.Validation("pick a product or enter a barcode")
	}
	if err := printing.Raw(c.Request().Context(), printer, tspl.Document(in)); err != nil {
		return apperr.Internal("could not print labels", err)
	}
	return c.NoContent(http.StatusNoContent)
}

// labelImage prints Image mode's label: when the request is the label as a
// picture (Content-Type image/png, drawn by the SPA at 8 dots per mm), it is
// printed as one TSPL bitmap, ?count times (1–maxCount) at ?size (blank = the
// shop's label), on the same label printer. done=false: not a picture, the
// caller prints Classic.
func (h *handlers) labelImage(c echo.Context, maxCount int) (done bool, err error) {
	if !strings.HasPrefix(c.Request().Header.Get("Content-Type"), "image/png") {
		return false, nil
	}
	ctx := c.Request().Context()
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return true, err
	}
	w, hgt := labelSize(c.QueryParam("size"), cfg.LabelWidthMM, cfg.LabelHeightMM)
	count, _ := strconv.Atoi(c.QueryParam("count"))
	img, err := escpos.ReadImage(c.Request().Body, w*8)
	var job []byte
	if err == nil {
		job, err = tspl.ImageDocument(img, w, hgt, min(max(cfg.LabelGapMM, 0), 20), min(max(count, 1), maxCount))
	}
	if errors.Is(err, escpos.ErrBadImage) || errors.Is(err, tspl.ErrBadImage) {
		return true, &apperr.AppError{Code: "BAD_IMAGE", Message: "That label picture can't be printed", Status: http.StatusUnprocessableEntity}
	}
	if err != nil {
		return true, err
	}
	if err := printing.Raw(ctx, cfg.LabelPrinter, job); err != nil {
		return true, apperr.Internal("could not print labels", err)
	}
	return true, c.NoContent(http.StatusNoContent)
}

// textLabelInput reads a text-only label request at the shop's label size.
func (h *handlers) textLabelInput(c echo.Context) (tspl.Input, string, error) {
	var req TextLabelRequest
	if err := c.Bind(&req); err != nil {
		return tspl.Input{}, "", apperr.BadRequest("invalid request body")
	}
	var lines []string
	for _, l := range req.Lines {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 || len(lines) > 12 {
		return tspl.Input{}, "", apperr.Validation("a label needs 1 to 12 lines of text")
	}
	cfg, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return tspl.Input{}, "", err
	}
	w, hgt := labelSize(req.Size, cfg.LabelWidthMM, cfg.LabelHeightMM)
	return tspl.Input{Lines: lines, Count: min(max(req.Count, 1), 50), WidthMM: w, HeightMM: hgt, GapMM: min(max(cfg.LabelGapMM, 0), 20)}, cfg.LabelPrinter, nil
}

// CounterTextLabel prints a text-only label (no barcode) at the shop's label
// size — a plugin's dose sticker, a shelf note.
func (h *handlers) CounterTextLabel(c echo.Context) error {
	if done, err := h.labelImage(c, 50); done {
		return err
	}
	in, printer, err := h.textLabelInput(c)
	if err != nil {
		return err
	}
	if err := printing.Raw(c.Request().Context(), printer, tspl.Document(in)); err != nil {
		return apperr.Internal("could not print the label", err)
	}
	return c.NoContent(http.StatusNoContent)
}

// CounterTextLabelPreview is the text label's print layout (cut = lines dropped).
func (h *handlers) CounterTextLabelPreview(c echo.Context) error {
	in, _, err := h.textLabelInput(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, labelLayoutDTO(in))
}

// CounterLabelPreview returns exactly where the printer will put everything
// (tspl.LayoutOf, the same geometry Document prints), for a true-to-paper preview.
func (h *handlers) CounterLabelPreview(c echo.Context) error {
	in, _, err := h.labelInput(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, labelLayoutDTO(in))
}

// labelLayoutDTO is exactly where the printer puts everything (tspl.LayoutOf).
func labelLayoutDTO(in tspl.Input) LabelLayout {
	l := tspl.LayoutOf(in)
	out := LabelLayout{WidthMM: l.WidthMM, HeightMM: l.HeightMM, WidthDots: l.WidthDots, HeightDots: l.HeightDots, Texts: []LabelText{}, Cut: l.Cut,
		Barcode: LabelBarcode{X: l.Barcode.X, Y: l.Barcode.Y, Height: l.Barcode.Height, Narrow: l.Barcode.Narrow, Format: in.Format,
			Code: l.Barcode.Code, ReadableH: l.Barcode.ReadableH}}
	for _, t := range l.Texts {
		out.Texts = append(out.Texts, LabelText{X: t.X, Y: t.Y, CharW: t.CharW, GlyphH: t.GlyphH, Value: t.Value})
	}
	return out
}

// CounterLabelFields lists a product's custom fields flagged "print on label"
// (a plugin seam; [] with none), for the label's top/bottom pickers.
func (h *handlers) CounterLabelFields(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	out := []DetailRow{}
	if products.LabelFieldProvider != nil {
		for _, r := range products.LabelFieldProvider(c.Request().Context(), id) {
			out = append(out, DetailRow{Label: r.Label, Value: r.Value})
		}
	}
	return c.JSON(http.StatusOK, out)
}

// CounterAssignBarcode saves a (generated) barcode on a product that had none,
// so its label can print.
func (h *handlers) CounterAssignBarcode(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req AssignBarcodeRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.t.products.AssignBarcode(c.Request().Context(), id, strings.TrimSpace(req.Barcode)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "barcode assigned at the counter: "+req.Barcode)
	return c.NoContent(http.StatusNoContent)
}

// labelSize parses a "WxH" preset (mm), falling back to the shop default and
// clamping to what a label printer can take.
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
