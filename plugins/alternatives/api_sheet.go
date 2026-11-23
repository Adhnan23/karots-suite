package alternatives

import (
	"net/http"

	"karots-suite/internal/sheet"

	"github.com/labstack/echo/v4"
)

var sheetColumns = []string{"Group", "Tier", "Product", "Barcode"}

// GET /alternatives/sheet?format=csv|xlsx|ods — the whole set (ALT-9).
func (h handlers) sheetExport(c echo.Context) error {
	lines, err := h.p.store.SheetLines(c.Request().Context())
	if err != nil {
		return err
	}
	rows := make([][]string, len(lines))
	for i, l := range lines {
		rows[i] = []string{l.Group, l.Tier, l.Product, l.Barcode}
	}
	f := sheet.FormatFrom(c.QueryParam("format"))
	c.Response().Header().Set(echo.HeaderContentType, f.ContentType())
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="alternatives`+f.Ext()+`"`)
	c.Response().WriteHeader(http.StatusOK)
	return sheet.Write(c.Response(), f, sheetColumns, rows)
}
