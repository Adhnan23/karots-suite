package alternatives

import (
	"errors"
	"fmt"
	"net/http"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/sheet"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// SheetImportDTO is what an import did (or, with ?dry=1, would do). Skipped reads
// "line N: why". Mirrored in web/src/plugins/alternatives/types.ts.
type SheetImportDTO struct {
	Groups        int      `json:"groups"`
	Members       int      `json:"members"`
	GroupsAdded   int      `json:"groupsAdded"`
	GroupsRemoved int      `json:"groupsRemoved"`
	Placed        int      `json:"placed"`
	Moved         int      `json:"moved"`
	TakenOut      int      `json:"takenOut"`
	Removed       []string `json:"removed"`
	Skipped       []string `json:"skipped"`
	Applied       bool     `json:"applied"`
}

var sheetSynonyms = map[string]string{
	"group name": "group", "alternative": "group", "alternatives": "group",
	"quality": "tier", "level": "tier",
	"item": "product", "name": "product", "product name": "product",
	"code": "barcode", "sku": "barcode",
}

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

// errSheetPreview rolls a preview's transaction back once its result is known.
var errSheetPreview = errors.New("alternatives sheet preview")

// POST /alternatives/sheet/import[?dry=1] — the sheet becomes the whole set
// (ALT-10), previewed with dry=1 (ALT-11). Admin only (ALT-16).
func (h handlers) sheetImport(c echo.Context) error {
	ctx := c.Request().Context()
	fh, err := c.FormFile("file")
	if err != nil {
		return apperr.BadRequest("please choose a file to import")
	}
	f, err := fh.Open()
	if err != nil {
		return apperr.BadRequest("could not read the uploaded file")
	}
	defer f.Close()
	rows, err := sheet.Read(fh.Filename, f)
	if err != nil || len(rows) == 0 {
		return apperr.BadRequest("could not read the file — is it a valid CSV, Excel or ODF spreadsheet?")
	}
	col := sheet.Columns(rows[0], sheetSynonyms)
	if _, ok := col["group"]; !ok {
		return apperr.BadRequest("the file must have a 'Group' column")
	}
	var lines []SheetLine
	for i, rec := range rows[1:] {
		l := SheetLine{Line: i + 2, Group: sheet.Cell(col, rec, "group"), Tier: sheet.Cell(col, rec, "tier"),
			Product: sheet.Cell(col, rec, "product"), Barcode: sheet.Cell(col, rec, "barcode")}
		if l.Product != "" || l.Barcode != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return apperr.BadRequest("the sheet has no product lines")
	}
	dry := c.QueryParam("dry") == "1"
	var res SheetResult
	err = appdb.WithTx(ctx, h.p.core.DB, func(tx *sqlx.Tx) error {
		var rerr error
		if res, rerr = Rebuild(ctx, tx, lines, products.NewRepository(h.p.core.DB).FindForMenu); rerr != nil {
			return rerr
		}
		if dry {
			return errSheetPreview
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSheetPreview) {
		return err
	}
	out := SheetImportDTO{Groups: res.Groups, Members: res.Members, GroupsAdded: res.GroupsAdded, GroupsRemoved: res.GroupsRemoved,
		Placed: res.Placed, Moved: res.Moved, TakenOut: res.TakenOut, Removed: []string{}, Skipped: []string{}, Applied: !dry}
	out.Removed = append(out.Removed, res.Removed...)
	for _, k := range res.Skipped {
		out.Skipped = append(out.Skipped, fmt.Sprintf("line %d: %s", k.Line, k.Reason))
	}
	if !dry {
		h.audit(c, audit.ActionUpdate, "alt_group", 0, fmt.Sprintf("alternatives replaced from a sheet: %d groups, %d products", out.Groups, out.Members))
	}
	return c.JSON(http.StatusOK, out)
}
