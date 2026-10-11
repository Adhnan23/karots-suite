package api

import (
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// Products → Sheets options: ?disabled=1 exports disabled products too, with an
// "active" column; ?extras=<key,…> adds those plugins' columns
// (plugin.ProductSheetColumns) to the export, the template and the import. With
// neither, the sheet is exactly the core columns.

// sheetExtra is one requested plugin's columns.
type sheetExtra struct {
	hook plugin.ProductSheetColumns
	cols []string
}

// AdminProductSheetExtras lists the plugin column groups Sheets can toggle.
func (h *handlers) AdminProductSheetExtras(c echo.Context) error {
	out := []SheetExtra{}
	for _, x := range plugin.ProductSheetColumnsHooks() {
		out = append(out, SheetExtra{Key: x.Key, Label: x.Label})
	}
	return c.JSON(http.StatusOK, out)
}

// sheetExtras resolves ?extras= (an unknown key is a 400).
func sheetExtras(c echo.Context) ([]sheetExtra, error) {
	want := strings.TrimSpace(c.QueryParam("extras"))
	if want == "" {
		return nil, nil
	}
	hooks := map[string]plugin.ProductSheetColumns{}
	for _, x := range plugin.ProductSheetColumnsHooks() {
		hooks[x.Key] = x
	}
	var out []sheetExtra
	for _, key := range strings.Split(want, ",") {
		x, ok := hooks[strings.TrimSpace(key)]
		if !ok {
			return nil, apperr.BadRequest("unknown sheet columns '" + key + "'")
		}
		cols, err := x.Columns(c.Request().Context())
		if err != nil {
			return nil, err
		}
		out = append(out, sheetExtra{hook: x, cols: cols})
	}
	return out, nil
}

// productSheetHeader is the core columns, then "active" (disabled=1), then each
// extra's columns.
func productSheetHeader(withActive bool, xs []sheetExtra) []string {
	h := append([]string{}, productColumns...)
	if withActive {
		h = append(h, "active")
	}
	for _, x := range xs {
		h = append(h, x.cols...)
	}
	return h
}

// AdminProductsTemplate is the blank products sheet, with the requested options.
func (h *handlers) AdminProductsTemplate(c echo.Context) error {
	xs, err := sheetExtras(c)
	if err != nil {
		return err
	}
	return writeSheet(c, "products-template", productSheetHeader(c.QueryParam("disabled") == "1", xs), nil)
}

// extraCells appends each extra's cells for the given rows' products (blank when
// a plugin has none for a product).
func extraCells(c echo.Context, xs []sheetExtra, ids []int64, rows [][]string) error {
	for _, x := range xs {
		cells, err := x.hook.Export(c.Request().Context(), ids)
		if err != nil {
			return err
		}
		for i, id := range ids {
			got := cells[id]
			for j := range x.cols {
				v := ""
				if j < len(got) {
					v = got[j]
				}
				rows[i] = append(rows[i], v)
			}
		}
	}
	return nil
}

// importExtras hands one imported row's plugin cells to each extra; a failure or
// a bad cell comes back as a note on that line, the product stays imported.
func importExtras(c echo.Context, xs []sheetExtra, productID, categoryID int64, get func(string) string) string {
	var notes []string
	for _, x := range xs {
		cells := map[string]string{}
		for _, col := range x.cols {
			if v := get(strings.ToLower(col)); v != "" {
				cells[col] = v
			}
		}
		if len(cells) == 0 {
			continue
		}
		note, err := x.hook.Import(c.Request().Context(), productID, categoryID, cells)
		if err != nil {
			note = x.hook.Label + " not saved: " + importErrMsg(err)
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	return strings.Join(notes, "; ")
}
