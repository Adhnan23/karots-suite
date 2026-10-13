package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/categories"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/shopspring/decimal"
)

// Spreadsheet import / export for products, customers and suppliers (karots-pos
// import_*.go). The export uses the import's columns, so a sheet round-trips:
// export, edit, import. Each import is best-effort — good rows land, bad rows
// come back as "line N: reason".

var productColumns = []string{
	"name", "name_local", "barcode", "extra_barcodes", "category", "unit",
	"cost_price", "selling_price", "wholesale_price", "tax_rate",
	"reorder_level", "opening_qty", "supplier", "track_serial", "warranty_months",
}

var productSynonyms = map[string]string{
	"product": "name", "item": "name",
	"sinhala": "name_local", "tamil": "name_local", "local": "name_local",
	"local name": "name_local", "name (local)": "name_local", "name_si": "name_local",
	"code": "barcode", "sku": "barcode",
	"extra barcodes": "extra_barcodes", "other barcodes": "extra_barcodes", "barcodes": "extra_barcodes",
	"category path": "category", "categories": "category",
	"uom":  "unit",
	"cost": "cost_price", "buying price": "cost_price", "cost price": "cost_price",
	"price": "selling_price", "selling": "selling_price", "selling price": "selling_price", "retail": "selling_price",
	"wholesale": "wholesale_price", "wholesale price": "wholesale_price",
	"tax": "tax_rate", "vat": "tax_rate", "tax %": "tax_rate", "tax%": "tax_rate",
	"reorder": "reorder_level", "reorder level": "reorder_level",
	"opening": "opening_qty", "qty": "opening_qty", "quantity": "opening_qty", "stock": "opening_qty",
	"serial": "track_serial", "warranty": "warranty_months",
}

var customerColumns = []string{"name", "phone", "address", "credit_limit", "opening_balance"}

var customerSynonyms = map[string]string{
	"customer": "name", "customer name": "name", "full name": "name",
	"mobile": "phone", "contact": "phone", "tel": "phone", "telephone": "phone",
	"addr":   "address",
	"credit": "credit_limit", "credit limit": "credit_limit", "limit": "credit_limit",
	"balance": "opening_balance", "opening": "opening_balance",
	"opening balance": "opening_balance", "due": "opening_balance",
	"outstanding": "opening_balance", "owed": "opening_balance",
}

var supplierColumns = []string{"name", "contact_person", "phone", "address", "credit_days", "opening_balance"}

var supplierSynonyms = map[string]string{
	"supplier": "name", "vendor": "name", "supplier name": "name",
	"contact": "contact_person", "contact person": "contact_person", "person": "contact_person",
	"mobile": "phone", "tel": "phone", "telephone": "phone",
	"addr":   "address",
	"credit": "credit_days", "credit days": "credit_days", "terms": "credit_days", "days": "credit_days",
	"balance": "opening_balance", "opening": "opening_balance",
	"opening balance": "opening_balance", "payable": "opening_balance",
	"outstanding": "opening_balance", "owed": "opening_balance",
}

func (h *handlers) registerAdminImports(g *echo.Group) {
	limit := echomw.BodyLimit("8M")
	for _, e := range []struct {
		path    string
		columns []string
		export  echo.HandlerFunc
		imp     echo.HandlerFunc
	}{
		{"/products", productColumns, h.AdminProductsExport, h.AdminProductsImport},
		{"/customers", customerColumns, h.AdminCustomersExport, h.AdminCustomersImport},
		{"/suppliers", supplierColumns, h.AdminSuppliersExport, h.AdminSuppliersImport},
	} {
		name, cols := strings.TrimPrefix(e.path, "/"), e.columns
		tmpl := func(c echo.Context) error { return writeSheet(c, name+"-template", cols, nil) }
		if name == "products" {
			tmpl = h.AdminProductsTemplate
		}
		g.GET(e.path+"/export", e.export)
		g.GET(e.path+"/import/template", tmpl)
		g.POST(e.path+"/import", e.imp, limit)
	}
	g.GET("/products/sheet-extras", h.AdminProductSheetExtras)
}

func money2(d decimal.Decimal) string { return d.StringFixed(2) }

// importRows walks a sheet's data rows: blank rows are skipped silently, a row
// without a name is an error, and each other row goes to fn (an error from fn
// skips that row and is reported with its line). The header is line 1.
func importRows(recs [][]string, col map[string]int, res *EntityImportResult, fn func(get func(string) string) (action, note string, err error)) {
	for i, rec := range recs {
		line := i + 2
		if i >= maxImportRows {
			res.Notes = append(res.Notes, fmt.Sprintf("stopped at %d rows (the limit for one file)", maxImportRows))
			return
		}
		get := cellGetter(col, rec)
		if get("name") == "" {
			if strings.TrimSpace(strings.Join(rec, "")) != "" {
				res.Errors = append(res.Errors, ImportRowError{Line: line, Message: "no name"})
				res.Skipped++
			}
			continue
		}
		action, note, err := fn(get)
		switch {
		case err != nil:
			res.Errors = append(res.Errors, ImportRowError{Line: line, Message: importErrMsg(err)})
			res.Skipped++
			continue
		case action == "created":
			res.Created++
		default:
			res.Updated++
		}
		if note != "" {
			res.Notes = append(res.Notes, fmt.Sprintf("Line %d: %s", line, note))
		}
	}
}

// givenCols names the canonical columns this row really fills (in the header
// and not blank), so an update leaves the rest of the record alone.
func givenCols(col map[string]int, get func(string) string) map[string]bool {
	out := make(map[string]bool, len(col))
	for k := range col {
		if get(k) != "" {
			out[k] = true
		}
	}
	return out
}

// cellAmounts reads named amount columns, naming the first bad one.
func cellAmounts(get func(string) string, labels map[string]string, keys ...string) (map[string]decimal.Decimal, error) {
	out := make(map[string]decimal.Decimal, len(keys))
	for _, k := range keys {
		d, err := amountCell(get(k))
		if err != nil {
			return nil, apperr.Validation(fmt.Sprintf("%s %q is not a number", labels[k], get(k)))
		}
		out[k] = d
	}
	return out, nil
}

func newImportResult() EntityImportResult {
	return EntityImportResult{Errors: []ImportRowError{}, Notes: []string{}}
}

func (h *handlers) finishImport(c echo.Context, entity string, res EntityImportResult) error {
	h.audit(c, audit.ActionCreate, entity, "", fmt.Sprintf("sheet import: %d created, %d updated, %d skipped", res.Created, res.Updated, res.Skipped))
	return c.JSON(http.StatusOK, res)
}

// ---- products ----

func (h *handlers) AdminProductsExport(c echo.Context) error {
	ctx := c.Request().Context()
	xs, err := sheetExtras(c)
	if err != nil {
		return err
	}
	withDisabled := c.QueryParam("disabled") == "1"
	rows, err := h.t.products.ListAll(ctx, withDisabled)
	if err != nil {
		return err
	}
	// Full "Parent > Child" paths, so a nested product comes back to its place.
	path := map[int64]string{}
	if nodes, err := categories.NewService(h.d.DB).Tree(ctx); err == nil {
		name := make(map[int64]string, len(nodes))
		for _, n := range nodes {
			name[n.ID] = n.Name
		}
		for _, n := range nodes {
			parts := make([]string, 0, len(n.Path)+1)
			for _, a := range n.Path {
				parts = append(parts, name[a])
			}
			path[n.ID] = strings.Join(append(parts, n.Name), " > ")
		}
	}
	out := make([][]string, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.ID)
		cat := path[p.CategoryID]
		if cat == "" {
			cat = p.CategoryName
		}
		out = append(out, []string{
			p.Name, str(p.NameLocal), str(p.Barcode), strings.Join(p.Barcodes, ";"), cat, p.UnitAbbr,
			money2(p.CostPrice), money2(p.SellingPrice), money2(p.WholesalePrice), money2(p.TaxRate),
			fmt.Sprint(p.ReorderLevel), p.StockQty.String(), str(p.PreferredSupplierName),
			fmt.Sprint(p.TrackSerial), fmt.Sprint(p.WarrantyMonths),
		})
		if withDisabled {
			out[len(out)-1] = append(out[len(out)-1], map[bool]string{true: "yes", false: "no"}[p.IsActive])
		}
	}
	if err := extraCells(c, xs, ids, out); err != nil {
		return err
	}
	return writeSheet(c, "products", productSheetHeader(withDisabled, xs), out)
}

var productLabels = map[string]string{"cost_price": "cost price", "selling_price": "selling price",
	"wholesale_price": "wholesale price", "tax_rate": "tax rate", "opening_qty": "opening quantity"}

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
	cats, sups := categories.NewService(h.d.DB), suppliers.NewService(h.d.DB)
	matchByName := yesCell(c.FormValue("matchByName"))
	user := middleware.CurrentUserID(c)
	res := newImportResult()
	importRows(recs, col, &res, func(get func(string) string) (string, string, error) {
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
		r, err := h.t.products.ImportOne(ctx, products.ImportRow{
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
		if n := importExtras(c, xs, r.ProductID, catID, get); n != "" {
			r.Note = strings.TrimPrefix(r.Note+"; "+n, "; ")
		}
		return r.Action, r.Note, nil
	})
	return h.finishImport(c, "product", res)
}

// importBarcodes adds a row's extra barcodes (";" or "," between them); it never
// removes one. A code another product carries is a note for the report, not a
// failed row.
func (h *handlers) importBarcodes(ctx context.Context, productID int64, cell string) string {
	var notes []string
	for _, code := range strings.FieldsFunc(cell, func(r rune) bool { return r == ';' || r == ',' }) {
		if code = strings.TrimSpace(code); code == "" {
			continue
		}
		if err := h.t.products.AddBarcode(ctx, productID, code, "import"); err != nil {
			notes = append(notes, "barcode "+code+" skipped: "+err.Error())
		}
	}
	return strings.Join(notes, "; ")
}

// ---- customers ----

func (h *handlers) AdminCustomersExport(c echo.Context) error {
	rows, err := h.t.customers.AllActive(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([][]string, 0, len(rows))
	for _, cu := range rows {
		out = append(out, []string{cu.Name, str(cu.Phone), str(cu.Address), money2(cu.CreditLimit), money2(cu.OutstandingBalance)})
	}
	return writeSheet(c, "customers", customerColumns, out)
}

var peopleLabels = map[string]string{"credit_limit": "credit limit", "opening_balance": "opening balance"}

func (h *handlers) AdminCustomersImport(c echo.Context) error {
	ctx := c.Request().Context()
	col, recs, err := readImportSheet(c, customerSynonyms, "name")
	if err != nil {
		return err
	}
	res := newImportResult()
	importRows(recs, col, &res, func(get func(string) string) (string, string, error) {
		amt, err := cellAmounts(get, peopleLabels, "credit_limit", "opening_balance")
		if err != nil {
			return "", "", err
		}
		if amt["credit_limit"].IsNegative() || amt["opening_balance"].IsNegative() {
			return "", "", apperr.Validation("credit limit and opening balance can't be negative")
		}
		r, err := h.t.customers.ImportOne(ctx, customers.ImportRow{Name: get("name"), Phone: get("phone"), Address: get("address"),
			CreditLimit: amt["credit_limit"], OpeningBalance: amt["opening_balance"], Given: givenCols(col, get)})
		return r.Action, r.Note, err
	})
	return h.finishImport(c, "customer", res)
}

// ---- suppliers ----

func (h *handlers) AdminSuppliersExport(c echo.Context) error {
	rows, err := suppliers.NewService(h.d.DB).ListAll(c.Request().Context(), "")
	if err != nil {
		return err
	}
	out := make([][]string, 0, len(rows))
	for _, s := range rows {
		if !s.IsActive {
			continue
		}
		out = append(out, []string{s.Name, str(s.ContactPerson), str(s.Phone), str(s.Address), fmt.Sprint(s.CreditDays), money2(s.OutstandingBalance)})
	}
	return writeSheet(c, "suppliers", supplierColumns, out)
}

func (h *handlers) AdminSuppliersImport(c echo.Context) error {
	ctx := c.Request().Context()
	col, recs, err := readImportSheet(c, supplierSynonyms, "name")
	if err != nil {
		return err
	}
	svc := suppliers.NewService(h.d.DB)
	res := newImportResult()
	importRows(recs, col, &res, func(get func(string) string) (string, string, error) {
		amt, err := cellAmounts(get, peopleLabels, "opening_balance")
		if err != nil {
			return "", "", err
		}
		days, derr := wholeCell(get("credit_days"))
		if derr != nil || days < 0 || amt["opening_balance"].IsNegative() {
			return "", "", apperr.Validation("credit days must be a whole number and the opening balance not negative")
		}
		r, err := svc.ImportOne(ctx, suppliers.CreateInput{Name: get("name"), ContactPerson: nilIfEmpty(get("contact_person")),
			Phone: nilIfEmpty(get("phone")), Address: nilIfEmpty(get("address")), CreditDays: days}, amt["opening_balance"], givenCols(col, get))
		return r.Action, r.Note, err
	})
	return h.finishImport(c, "supplier", res)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
