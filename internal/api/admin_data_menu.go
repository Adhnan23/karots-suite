package api

import (
	"errors"
	"fmt"
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// The cashier menu sheet on Data → Spreadsheets (docs/rules/menu.md): the whole
// menu out, and a sheet in that becomes the whole menu — previewed first.

var menuColumns = []string{"Group", "Emoji", "Active", "Product", "Barcode", "Product emoji"}

var menuSynonyms = map[string]string{
	"category": "group", "menu": "group", "group path": "group",
	"item": "product", "name": "product",
	"code": "barcode", "sku": "barcode",
	"shown": "active", "visible": "active",
	"item emoji": "product emoji",
}

func (h *handlers) AdminMenuExport(c echo.Context) error {
	lines, err := productgroups.NewRepository(h.d.DB).MenuLines(c.Request().Context())
	if err != nil {
		return err
	}
	rows := make([][]string, len(lines))
	for i, l := range lines {
		rows[i] = []string{l.Group, l.Emoji, l.Active, l.Product, l.Barcode, l.ProductEmoji}
	}
	return writeSheet(c, "cashier-menu", menuColumns, rows)
}

// errMenuPreview rolls a preview's transaction back once its result is known.
var errMenuPreview = errors.New("menu preview")

func (h *handlers) AdminMenuImport(c echo.Context) error {
	ctx := c.Request().Context()
	col, recs, err := readImportSheet(c, menuSynonyms, "group")
	if err != nil {
		return err
	}
	lines := make([]productgroups.MenuLine, 0, len(recs))
	for i, rec := range recs {
		get := cellGetter(col, rec)
		l := productgroups.MenuLine{Line: i + 2, Group: get("group"), Emoji: get("emoji"), Active: get("active"),
			Product: get("product"), Barcode: get("barcode"), ProductEmoji: get("product emoji")}
		if l != (productgroups.MenuLine{Line: l.Line}) {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return apperr.BadRequest("the sheet has no menu lines")
	}
	dry := c.QueryParam("dry") == "1"
	var res productgroups.MenuResult
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		var rerr error
		if res, rerr = productgroups.Rebuild(ctx, tx, lines, products.NewRepository(h.d.DB).FindForMenu); rerr != nil {
			return rerr
		}
		if dry {
			return errMenuPreview
		}
		return nil
	})
	if err != nil && !errors.Is(err, errMenuPreview) {
		return err
	}
	out := MenuImportResult{Groups: res.Groups, Items: res.Items, RemovedGroups: res.RemovedGroups,
		RemovedItems: res.RemovedItems, Skipped: []string{}, Applied: !dry}
	for _, s := range res.Skipped {
		out.Skipped = append(out.Skipped, fmt.Sprintf("line %d: %s", s.Line, s.Reason))
	}
	if !dry {
		h.audit(c, audit.ActionUpdate, "product_group", "", fmt.Sprintf("cashier menu replaced from a sheet: %d groups, %d items", out.Groups, out.Items))
	}
	return c.JSON(http.StatusOK, out)
}
