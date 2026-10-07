package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerReportsStock(g *echo.Group) {
	g.GET("/reports/inventory", h.ReportInventory)
	g.GET("/reports/inventory/products", h.ReportInventoryProducts)
	g.GET("/reports/count-sheet", h.ReportCountSheet)
	g.GET("/reports/topup", h.ReportTopup)
}

// idParam is a positive id query param, or nil.
func idParam(c echo.Context, name string) *int64 {
	if id, err := strconv.ParseInt(c.QueryParam(name), 10, 64); err == nil && id > 0 {
		return &id
	}
	return nil
}

func valuationQuery(c echo.Context) products.ValuationQuery {
	return products.ValuationQuery{CategoryID: idParam(c, "category"), IncludeZero: c.QueryParam("zero") == "1"}
}

func inventoryProduct(p products.ValuedProduct) InventoryProduct {
	return InventoryProduct{ID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode), Category: p.Category, Unit: p.Unit, Qty: p.Qty.String(),
		CostValue: dec2(p.CostValue), RetailValue: dec2(p.RetailValue)}
}

// ReportInventory values one branch of the catalogue (?category=, none = the
// whole shop) from SQL over every product; ?format= downloads every product.
func (h *handlers) ReportInventory(c echo.Context) error {
	ctx := c.Request().Context()
	q := valuationQuery(c)
	if wantsSheet(c) {
		rows, err := h.t.products.ValuedProducts(ctx, q, 0, 0)
		if err != nil {
			return err
		}
		sheet := make([][]string, 0, len(rows))
		for _, p := range rows {
			x := inventoryProduct(p)
			sheet = append(sheet, []string{x.Name, x.Barcode, x.Category, x.Unit, x.Qty, x.CostValue, x.RetailValue})
		}
		return writeSheet(c, "inventory_valuation_"+datetime.Date(time.Now()), []string{"Product", "Barcode", "Category", "Unit", "On hand", "Cost value", "Retail value"}, sheet)
	}
	v, err := h.t.products.Valuation(ctx, q.CategoryID)
	if err != nil {
		return err
	}
	out := InventoryReport{SKUs: v.SKUs, InStock: v.InStock, Units: []UnitAmount{}, CostValue: dec2(v.CostValue), RetailValue: dec2(v.RetailValue),
		Margin: dec2(v.Margin()), MarginPct: v.MarginPct().StringFixed(1), MissingCost: v.MissingCost, NegativeQty: v.NegativeQty,
		Breadcrumb: []Crumb{}, Children: make([]InventoryNode, 0, len(v.Children))}
	for _, u := range v.Units {
		out.Units = append(out.Units, UnitAmount{Unit: u.Abbr, Qty: u.Qty.String()})
	}
	for _, b := range v.Breadcrumb {
		out.Breadcrumb = append(out.Breadcrumb, Crumb{ID: b.ID, Name: b.Name})
	}
	for _, n := range v.Children {
		out.Children = append(out.Children, InventoryNode{ID: n.CategoryID, Name: n.Name, Direct: n.Direct, HasChildren: n.HasChildren, SKUs: n.SKUs,
			InStock: n.InStock, CostValue: dec2(n.CostValue), RetailValue: dec2(n.RetailValue)})
	}
	return c.JSON(http.StatusOK, out)
}

// ReportInventoryProducts pages the branch's products, most valuable first.
func (h *handlers) ReportInventoryProducts(c echo.Context) error {
	rows, err := h.t.products.ValuedProducts(c.Request().Context(), valuationQuery(c), counterPage, listOffset(c))
	if err != nil {
		return err
	}
	out := make([]InventoryProduct, 0, len(rows))
	for _, p := range rows {
		out = append(out, inventoryProduct(p))
	}
	return c.JSON(http.StatusOK, out)
}

// ReportCountSheet is the shelf-count worklist (screen and A4 print share it).
func (h *handlers) ReportCountSheet(c echo.Context) error {
	ctx := c.Request().Context()
	cat, sup := idParam(c, "category"), idParam(c, "supplier")
	rows, err := h.t.products.CountSheet(ctx, cat, sup)
	if err != nil {
		return err
	}
	out := CountSheet{Rows: make([]CountSheetLine, 0, len(rows))}
	if cat != nil {
		if path, err := h.t.products.CategoryPath(ctx, *cat); err == nil && len(path) > 0 {
			out.Category = path[len(path)-1].Name
		}
	}
	if sup != nil {
		if s, err := suppliers.NewService(h.d.DB).Get(ctx, *sup); err == nil {
			out.Supplier = s.Name
		}
	}
	for _, r := range rows {
		out.Rows = append(out.Rows, CountSheetLine{ID: r.ID, Name: r.Name, Barcode: strOr(r.Barcode), Unit: r.Unit, Category: r.CategoryPath, OnHand: r.StockQty.String()})
	}
	return c.JSON(http.StatusOK, out)
}

// ReportTopup is Top-up & reorder (karots-pos): plugin readiness (empty in a
// core build) and stock under its reorder level (?category=, with its
// sub-categories), against every active locker and open till.
func (h *handlers) ReportTopup(c echo.Context) error {
	ctx := c.Request().Context()
	out := TopupReport{Plugins: []ReadinessGroup{}, Stock: []TopupStockRow{}, Physical: []PlaceBalance{}, Bank: []PlaceBalance{}}
	var pluginNeed, stockNeed, physical, bank decimal.Decimal
	for _, s := range plugin.CollectReadiness(ctx) {
		g := ReadinessGroup{Title: s.Title, Need: dec2(s.NeedTotal), Rows: make([]ReadinessLine, 0, len(s.Rows))}
		for _, r := range s.Rows {
			g.Rows = append(g.Rows, ReadinessLine{Name: r.Name, Have: r.Have, Target: r.Target, Need: dec2(r.Need)})
		}
		out.Plugins = append(out.Plugins, g)
		pluginNeed = pluginNeed.Add(s.NeedTotal)
	}
	needs, err := reports.NewService(h.d.DB).ReorderNeeds(ctx, idParam(c, "category"))
	if err != nil {
		return err
	}
	for _, n := range needs {
		out.Stock = append(out.Stock, TopupStockRow{ProductID: n.ProductID, Category: n.Category, Product: n.Product, OnHand: n.OnHand.String(),
			Reorder: n.Reorder, Short: n.Shortfall().String(), Cost: dec2(n.CostPrice), Restock: dec2(n.RestockCost())})
		stockNeed = stockNeed.Add(n.RestockCost())
	}
	lks, err := h.t.lockers.List(ctx, true)
	if err != nil {
		return err
	}
	for _, l := range lks {
		p := PlaceBalance{Name: l.Name, Balance: dec2(l.Balance)}
		if l.Kind == lockers.KindBank || l.Kind == lockers.KindCard {
			out.Bank, bank = append(out.Bank, p), bank.Add(l.Balance)
		} else {
			out.Physical, physical = append(out.Physical, p), physical.Add(l.Balance)
		}
	}
	tills, err := h.d.CashRegister.OpenSessions(ctx)
	if err != nil {
		return err
	}
	for _, t := range tills {
		bal := t.OpeningCash
		if sum, err := h.d.CashRegister.Summary(ctx, t.UserID); err == nil {
			bal = sum.Expected
		}
		out.Physical, physical = append(out.Physical, PlaceBalance{Name: "Till — " + t.UserName, Balance: dec2(bal)}), physical.Add(bal)
	}
	need, avail := pluginNeed.Add(stockNeed), physical.Add(bank)
	out.PluginNeed, out.StockNeed, out.TotalNeed = dec2(pluginNeed), dec2(stockNeed), dec2(need)
	out.PhysicalTotal, out.BankTotal, out.Available, out.Gap = dec2(physical), dec2(bank), dec2(avail), dec2(avail.Sub(need))
	return c.JSON(http.StatusOK, out)
}
