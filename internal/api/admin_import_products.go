package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/features/categories"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// The products sheet import (docs/rules/stock.md IMP): one transaction, a
// savepoint per line, a preview (?dry=1) that is the same run rolled back, and
// each line's changes, old → new.

// errSheetPreview rolls a products sheet preview back once its result is known.
var errSheetPreview = errors.New("sheet preview")

func (h *handlers) AdminProductsImport(c echo.Context) error {
	ctx := c.Request().Context()
	col, recs, err := readImportSheet(c, productSynonyms, "name")
	if err != nil {
		return err
	}
	xs, err := sheetExtras(c)
	if err != nil {
		return err
	}
	unitList, err := h.t.units.List(ctx)
	if err != nil {
		return err
	}
	if len(unitList) == 0 {
		return apperr.Validation("add at least one unit before importing products")
	}
	unitBy := map[string]int64{}
	for _, u := range unitList {
		unitBy[strings.ToLower(u.Name)] = u.ID
		unitBy[strings.ToLower(u.Abbreviation)] = u.ID
	}
	sups := suppliers.NewService(h.d.DB)
	matchByName := yesCell(c.FormValue("matchByName"))
	install, err := h.installID(c)
	if err != nil {
		return err
	}
	user := middleware.CurrentUserID(c)
	var lastID int64 // the product the last line touched (IMP-4)
	importProductRow := func(ctx context.Context, get func(string) string) (string, string, error) {
		cats := categories.NewService(db.TxFrom(ctx))
		unit := unitList[0].ID
		if u := get("unit"); u != "" {
			id, ok := unitBy[strings.ToLower(u)]
			if !ok {
				return "", "", apperr.Validation("unknown unit '" + u + "'")
			}
			unit = id
		}
		amt, err := cellAmounts(get, productLabels, "cost_price", "selling_price", "wholesale_price", "tax_rate", "opening_qty")
		if err != nil {
			return "", "", err
		}
		reorder, rerr := wholeCell(get("reorder_level"))
		warranty, werr := wholeCell(get("warranty_months"))
		if rerr != nil || werr != nil {
			return "", "", apperr.Validation("reorder level and warranty months must be whole numbers")
		}
		path := get("category")
		if path == "" {
			path = "Uncategorized"
		}
		catID, err := cats.FindOrCreateByPath(ctx, path)
		if err != nil || catID == 0 {
			return "", "", apperr.Validation("could not use the category '" + path + "'")
		}
		supID, err := sups.FindOrCreateByName(ctx, get("supplier"))
		if err != nil {
			return "", "", err
		}
		var active *bool
		if a := get("active"); a != "" {
			v := yesCell(a)
			active = &v
		}
		byID, err := refID(get("ref"), install)
		if err != nil {
			return "", "", err
		}
		r, err := h.t.products.ImportOne(ctx, products.ImportRow{
			ID:   byID,
			Name: get("name"), NameLocal: get("name_local"), Barcode: get("barcode"),
			CategoryID: catID, UnitID: unit, UserID: user, PreferredSupplier: supID,
			Cost: amt["cost_price"], Selling: amt["selling_price"], Wholesale: amt["wholesale_price"], Tax: amt["tax_rate"],
			Reorder: reorder, WarrantyMonths: warranty, TrackSerial: yesCell(get("track_serial")),
			OpeningQty: amt["opening_qty"], MatchByName: matchByName, Given: givenCols(col, get), Active: active,
		})
		if err != nil {
			return "", "", err
		}
		if n := h.importBarcodes(ctx, r.ProductID, get("extra_barcodes")); n != "" {
			r.Note = strings.TrimPrefix(r.Note+"; "+n, "; ")
		}
		if n := importExtras(ctx, xs, r.ProductID, catID, get); n != "" {
			r.Note = strings.TrimPrefix(r.Note+"; "+n, "; ")
		}
		lastID = r.ProductID
		return r.Action, r.Note, nil
	}
	dry := c.QueryParam("dry") == "1"
	var res EntityImportResult
	// IMP-1, 3: the whole sheet in one transaction, each line in a savepoint (a
	// line that fails leaves nothing behind); a preview is the same run, rolled back.
	err = db.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		tctx := db.InTx(ctx, tx)
		res = newImportResult()
		res.Changes = []ImportChange{}
		snaps, err := productSnaps(tctx, tx, nil, xs) // every product as it is now: a line's "before"
		if err != nil {
			return err
		}
		importRows(recs, col, &res, func(line int, get func(string) string) (action, note string, err error) {
			err = db.WithTx(tctx, h.d.DB, func(*sqlx.Tx) error {
				var rerr error
				if action, note, rerr = importProductRow(tctx, get); rerr != nil {
					return rerr
				}
				after, rerr := productSnaps(tctx, tx, []int64{lastID}, xs)
				if rerr != nil {
					return rerr
				}
				before, now := snaps[lastID], after[lastID]
				ch := []FieldChange{}
				if before != nil {
					ch = diffSnaps(before, now, xs)
				}
				if before == nil || len(ch) > 0 {
					res.Changes = append(res.Changes, ImportChange{Line: line, Product: now["name"], New: before == nil, Fields: ch})
				}
				snaps[lastID] = now // a product named again later starts from here
				return nil
			})
			return
		})
		if dry {
			return errSheetPreview
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSheetPreview) {
		return err
	}
	if dry {
		return c.JSON(http.StatusOK, res)
	}
	res.Applied = true
	return h.finishImport(c, "product", res)
}
