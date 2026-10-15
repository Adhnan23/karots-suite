package api

import (
	"context"
	"fmt"
	"karots-suite/internal/features/sales"
	"net/http"
	"strconv"

	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerTillCatalog(g *echo.Group) {
	g.GET("/config", h.TillConfig)
	g.GET("/groups", h.TillGroups)
	g.GET("/groups/:id", h.TillGroup)
	g.GET("/products", h.TillProducts)
	g.GET("/products/barcode/:code", h.TillProductByBarcode)
	g.GET("/products/:id", h.TillProduct)
	g.GET("/products/:id/detail", h.TillProductDetail)
	g.GET("/products/:id/lots", h.TillLots)
	g.GET("/price-options", h.TillPriceOptions)
	g.GET("/quick-picks", h.TillQuickPicks)
	g.GET("/units", h.TillUnits)
	g.POST("/products/:id/packs", h.TillPackCreate)
}

func (h *handlers) TillConfig(c echo.Context) error {
	ctx := c.Request().Context()
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	role := middleware.CurrentRole(c)
	out := TillConfig{
		Symbol:             st.CurrencySymbol,
		DefaultSaleType:    "retail",
		AskToPrint:         st.AskToPrint,
		OpenCashDrawer:     st.OpenCashDrawer,
		AllowUntrackedCash: st.AllowUntrackedCash,
		AllowContinueLast:  st.AllowContinueLastCount,
		CanManageCredit:    middleware.MayManageCredit(role, middleware.CanManageCredit(c)),
		CanHandleSuppliers: middleware.MaySeeSuppliers(role, middleware.CanHandleSuppliers(c)),
		CanSeeCost:         middleware.MaySeeCost(role, middleware.CanSeeCost(c)),
		CanChangePin:       role != auth.RoleCashier || h.d.Auth.AllowCashierPinChange(ctx),
		IsManager:          role == auth.RoleAdmin || role == auth.RoleManager,
		TenderMethods:      []TenderMethod{},
		LabelSize:          fmt.Sprintf("%dx%d", st.LabelWidthMM, st.LabelHeightMM),
		ReceiptWidth:       st.ReceiptWidth,
		Shop:               ShopHeader{Name: st.ShopName, Address: strOr(st.Address), Phone: strOr(st.Phone), Footer: strOr(st.ReceiptFooter)},
	}
	// A DB predating credit-as-a-payment may still hold 'credit' here; fall back
	// to retail so the till stays usable (karots-pos cashier.POS did the same).
	if st.DefaultSaleType == "wholesale" {
		out.DefaultSaleType = "wholesale"
	}
	if st.DefaultLockerID != nil {
		out.DefaultLockerID = *st.DefaultLockerID
	}
	for _, t := range plugin.TenderMethods() {
		out.TenderMethods = append(out.TenderMethods, TenderMethod{Value: t.Value, Label: t.Label, Hidden: t.Hidden})
	}
	return c.JSON(http.StatusOK, out)
}

func tillProduct(p products.Product) TillProduct {
	out := TillProduct{
		ID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode),
		SellingPrice: dec2(p.SellingPrice), WholesalePrice: dec2(p.WholesalePrice),
		TaxRate: p.TaxRate.String(), StockQty: p.StockQty.String(),
		UnitAbbr: p.UnitAbbr, UnitAllowDecimal: p.UnitAllowDecimal,
		IsService: p.IsService, TrackSerial: p.TrackSerial, WarrantyMonths: p.WarrantyMonths,
		Badges: p.Badges, Packs: []TillPack{},
		LowStock: !p.IsService && p.ReorderLevel > 0 && p.IsLowStock(),
	}
	if out.Badges == nil {
		out.Badges = []string{}
	}
	if s := p.Suggestion; s != nil {
		out.Suggestion = &SaleSuggestion{DiscountType: s.DiscountType, DiscountValue: s.DiscountValue, Label: s.Label, Prompt: s.Prompt}
	}
	return out
}

func tillProducts(rows []products.Product) []TillProduct {
	out := make([]TillProduct, 0, len(rows))
	for _, p := range rows {
		out = append(out, tillProduct(p))
	}
	return out
}

// withPacks fills each product's packs (one query for the page), and drops the
// tax rates when the shop isn't registered for tax — the sale won't charge them
// (sales.TaxOnSQL), so the till's total must not show them either.
func (h *handlers) withPacks(ctx context.Context, ps []TillProduct) []TillProduct {
	var taxOn bool
	_ = h.d.DB.GetContext(ctx, &taxOn, sales.TaxOnSQL) // no settings row = off
	for i := range ps {
		if !taxOn {
			ps[i].TaxRate = "0"
		}
	}
	ids := make([]int64, len(ps))
	for i, p := range ps {
		ids[i] = p.ID
		ps[i].Packs = []TillPack{}
	}
	m, err := products.PacksFor(ctx, h.d.DB, ids)
	if err != nil {
		return ps // the till still sells by the unit
	}
	for i := range ps {
		for _, k := range m[ps[i].ID] {
			ps[i].Packs = append(ps[i].Packs, TillPack{ID: k.ID, Name: k.Name, Size: k.Size.String(),
				SellingPrice: dec2(k.SellingPrice), WholesalePrice: dec2(k.WholesalePrice)})
		}
	}
	return ps
}

func groupProduct(p productgroups.GroupProduct) TillProduct {
	return TillProduct{
		ID: p.ProductID, Name: p.Name, Barcode: strOr(p.Barcode),
		SellingPrice: dec2(p.SellingPrice), WholesalePrice: dec2(p.WholesalePrice),
		TaxRate: p.TaxRate.String(), StockQty: p.StockQty.String(),
		UnitAbbr: p.UnitAbbr, UnitAllowDecimal: p.UnitAllowDecimal,
		IsService: p.IsService, TrackSerial: p.TrackSerial, WarrantyMonths: p.WarrantyMonths,
		Emoji: strOr(p.Emoji), Badges: []string{}, Packs: []TillPack{},
		LowStock: !p.IsService && p.ReorderLevel > 0 && p.StockQty.LessThanOrEqual(decimal.NewFromInt(int64(p.ReorderLevel))),
	}
}

func tillGroups(rows []productgroups.Group) []TillGroup {
	out := make([]TillGroup, 0, len(rows))
	for _, g := range rows {
		out = append(out, TillGroup{ID: g.ID, Name: g.Name, Emoji: strOr(g.Emoji), HasChildren: g.HasChildren, ItemCount: g.ItemCount})
	}
	return out
}

func lotOf(o stock.PriceOption) Lot {
	l := Lot{BatchID: o.BatchID, BatchNo: strOr(o.BatchNo), QtyRemaining: o.QtyRemaining.String(),
		Price: dec2(o.Price), ReceivedAt: o.ReceivedAt.Format("2006-01-02")}
	if o.ExpiryDate != nil {
		l.ExpiryDate = o.ExpiryDate.Format("2006-01-02")
	}
	return l
}

func lotsOf(rows []stock.PriceOption) []Lot {
	out := make([]Lot, 0, len(rows))
	for _, o := range rows {
		out = append(out, lotOf(o))
	}
	return out
}

func (h *handlers) TillGroups(c echo.Context) error {
	rows, err := h.t.groups.Children(c.Request().Context(), nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tillGroups(rows))
}

func (h *handlers) TillGroup(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	g, err := h.t.groups.Get(ctx, id)
	if err != nil {
		return err
	}
	children, err := h.t.groups.Children(ctx, &id)
	if err != nil {
		return err
	}
	prods, err := h.t.groups.Products(ctx, id)
	if err != nil {
		return err
	}
	crumb, err := h.t.groups.Breadcrumb(ctx, id)
	if err != nil {
		return err
	}
	out := GroupView{Group: tillGroups([]productgroups.Group{*g})[0], Breadcrumb: tillGroups(crumb),
		Children: tillGroups(children), Products: make([]TillProduct, 0, len(prods))}
	for _, p := range prods {
		out.Products = append(out.Products, groupProduct(p))
	}
	// Group cards carry plugin pins like search results do (a till menu root
	// claims a product by one).
	if products.BadgeProvider != nil && len(prods) > 0 {
		ids := make([]int64, len(prods))
		for i, p := range prods {
			ids[i] = p.ProductID
		}
		if m := products.BadgeProvider(ctx, ids); m != nil {
			for i := range out.Products {
				if b := m[out.Products[i].ID]; b != nil {
					out.Products[i].Badges = b
				}
			}
		}
	}
	out.Products = h.withPacks(ctx, out.Products)
	return c.JSON(http.StatusOK, out)
}

// TillProducts is the typed search behind the scan/search box (name, barcode,
// fuzzy rescue — all in products.List). Same 100-row cap as karots-pos.
// ?services=1 (the Sell screen) also finds services; stock screens leave it off.
func (h *handlers) TillProducts(c echo.Context) error {
	ctx := c.Request().Context()
	q := products.ListQuery{Page: 1, Limit: 100, Search: c.QueryParam("search"), IncludeServices: c.QueryParam("services") == "1"}
	q.Normalize()
	rows, total, err := h.t.products.List(ctx, q)
	if err != nil {
		return err
	}
	products.Decorate(ctx, rows)
	return c.JSON(http.StatusOK, ProductPage{Items: h.withPacks(ctx, tillProducts(rows)), Total: total})
}

func (h *handlers) TillProductByBarcode(c echo.Context) error {
	ctx := c.Request().Context()
	p, err := h.t.products.GetByBarcode(ctx, c.Param("code"))
	if err != nil {
		return err
	}
	h.t.products.BarcodeScanned(ctx, c.Param("code"))
	one := []products.Product{*p}
	products.Decorate(ctx, one)
	out := h.withPacks(ctx, []TillProduct{tillProduct(one[0])})[0]
	if _, packID, err := products.PackForBarcode(ctx, h.d.DB, c.Param("code")); err == nil {
		out.ScannedPackID = packID
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) TillProduct(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	one := []products.Product{*p}
	products.Decorate(ctx, one)
	return c.JSON(http.StatusOK, h.withPacks(ctx, []TillProduct{tillProduct(one[0])})[0])
}

// TillProductDetail is the ⓘ popup. Cost/margin only for users allowed to see
// cost (role-inclusive, like every other cost gate).
func (h *handlers) TillProductDetail(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	d := ProductDetail{ID: p.ID, Name: p.Name, Category: p.CategoryName, Barcode: strOr(p.Barcode), Unit: p.UnitAbbr,
		SellingPrice: dec2(p.SellingPrice), StockQty: p.StockQty.String(), Rows: []DetailRow{}}
	if p.WarrantyMonths > 0 {
		d.Warranty = strconv.Itoa(p.WarrantyMonths) + " month warranty"
	}
	if middleware.MaySeeCost(middleware.CurrentRole(c), middleware.CanSeeCost(c)) {
		d.ShowCost = true
		d.CostPrice = dec2(p.CostPrice)
		if p.SellingPrice.IsPositive() && p.CostPrice.IsPositive() {
			d.MarginPct = p.SellingPrice.Sub(p.CostPrice).Div(p.SellingPrice).Mul(decimal.NewFromInt(100)).StringFixed(1)
		}
	}
	if products.DetailContributor != nil {
		for _, r := range products.DetailContributor(ctx, p.ID) {
			d.Rows = append(d.Rows, DetailRow{Label: r.Label, Value: r.Value})
		}
	}
	return c.JSON(http.StatusOK, d)
}

func (h *handlers) TillLots(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	rows, err := h.t.products.LotsFor(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, lotsOf(rows))
}

// TillPriceOptions lists the FEW products whose live lots disagree on price or
// whose FEFO-first lot is expired — the only ones the till asks "which price?".
func (h *handlers) TillPriceOptions(c echo.Context) error {
	m, err := h.t.products.PriceOptions(c.Request().Context())
	if err != nil {
		return err
	}
	out := make(map[string][]Lot, len(m))
	for pid, rows := range m {
		out[strconv.FormatInt(pid, 10)] = lotsOf(rows)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) TillQuickPicks(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.t.products.QuickPicks(ctx)
	if err != nil {
		return err
	}
	products.Decorate(ctx, rows)
	return c.JSON(http.StatusOK, h.withPacks(c.Request().Context(), tillProducts(rows)))
}

func (h *handlers) TillUnits(c echo.Context) error {
	rows, err := h.t.units.List(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]Unit, 0, len(rows))
	for _, u := range rows {
		out = append(out, Unit{ID: u.ID, Name: u.Name, Abbreviation: u.Abbreviation, AllowDecimal: u.AllowDecimal})
	}
	return c.JSON(http.StatusOK, out)
}
