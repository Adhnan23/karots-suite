package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/printmachines"
	"karots-suite/internal/printing"
)

// Appearance prints what its previews show, so a style can be judged on paper
// before it's saved. A try-out: no sale, no drawer, and the choice of printer isn't kept.
func (h *handlers) registerSystemPrint(g *echo.Group) {
	g.GET("/printers", h.SystemPrinters)
	g.POST("/print/slip", h.SystemPrintSlip)
	g.POST("/print/label", h.SystemPrintLabel)
}

// SystemPrinters lists this PC's printers and those of the approved cashier PCs.
func (h *handlers) SystemPrinters(c echo.Context) error {
	ctx := c.Request().Context()
	s, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	out := SystemPrinterList{ReceiptPrinter: s.ReceiptPrinter, LabelPrinter: s.LabelPrinter, Printers: []SystemPrinter{}}
	for _, q := range printing.Queues(ctx) {
		out.Printers = append(out.Printers, SystemPrinter{Label: q, Target: q})
	}
	machines, err := h.machines.List(ctx)
	if err != nil {
		return err
	}
	for _, m := range machines {
		if m.Approved {
			for _, q := range m.PrinterNames() {
				out.Printers = append(out.Printers, SystemPrinter{Label: q + " on " + m.Name, Target: printmachines.Target(m.ID, q)})
			}
		}
	}
	return c.JSON(http.StatusOK, out)
}

// SystemPrintSlip prints the picture the page sends, on ?printer= or the shop's receipt printer.
func (h *handlers) SystemPrintSlip(c echo.Context) error {
	s, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	job, ok, err := slipImage(c, s)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation("send the receipt as a picture")
	}
	target := strings.TrimSpace(c.QueryParam("printer"))
	if target == "" {
		target = s.ReceiptPrinter
	}
	if strings.TrimSpace(target) == "" {
		return apperr.Validation("pick a printer first")
	}
	if err := printing.Raw(c.Request().Context(), target, job); err != nil {
		return apperr.Validation("it didn't print: " + err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

// SystemPrintLabel prints one label picture, on ?printer= or the shop's label printer.
func (h *handlers) SystemPrintLabel(c echo.Context) error {
	done, err := h.labelImageOn(c, 1, strings.TrimSpace(c.QueryParam("printer")))
	if !done {
		return apperr.Validation("send the label as a picture")
	}
	return err
}
