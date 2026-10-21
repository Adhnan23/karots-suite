package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

const productPageSize = 25

func (h *handlers) registerAdminProducts(g *echo.Group) {
	g.GET("/products", h.AdminProducts)
	g.GET("/products/review", h.AdminProductReview)
	g.GET("/products/review/count", h.AdminProductReviewCount)
	g.GET("/products/:id", h.AdminProduct)
	g.POST("/products", h.AdminProductCreate)
	g.PUT("/products/:id", h.AdminProductUpdate)
	g.DELETE("/products/:id", h.AdminProductDisable)
	g.POST("/products/:id/activate", h.AdminProductReactivate)
	g.POST("/products/:id/loose", h.AdminProductLoose)
	g.POST("/products/:id/fold", h.AdminProductFold)
	g.POST("/products/:id/review-done", h.AdminProductReviewDone)
	g.POST("/products/:id/barcode", h.CounterAssignBarcode) // same rule as the counter: only when it has none
	g.GET("/products/:id/barcodes", h.AdminBarcodes)
	g.POST("/products/:id/barcodes", h.AdminBarcodeAdd)
	g.DELETE("/products/:id/barcodes/:code", h.AdminBarcodeRemove)
	g.POST("/products/:id/barcodes/:code/main", h.AdminBarcodeMain)
	g.GET("/products/:id/merge/:into", h.AdminMergePreview)
	g.POST("/products/:id/merge/:into", h.AdminMerge)
	g.GET("/barcode/generate", h.TillGenerateBarcode)
	g.GET("/supplier-options", h.AdminSupplierOptions)
}

func productRow(p products.Product) AdminProductRow {
	return AdminProductRow{
		ID: p.ID, Name: p.Name, NameLocal: strOr(p.NameLocal), Barcode: strOr(p.Barcode), Barcodes: barcodesOf(p), CategoryID: p.CategoryID, CategoryName: p.CategoryName,
		UnitAbbr: p.UnitAbbr, CostPrice: dec2(p.CostPrice), SellingPrice: dec2(p.SellingPrice), Wholesale: dec2(p.WholesalePrice),
		StockQty: p.StockQty.String(), ReorderLevel: p.ReorderLevel, ReorderAuto: p.ReorderAuto, Low: !p.IsService && p.IsLowStock(), IsActive: p.IsActive, IsService: p.IsService,
		NeedsReview: p.NeedsReview, TrackSerial: p.TrackSerial, HasExpiry: p.HasExpiry, SupplierName: strOr(p.PreferredSupplierName),
		CreatedBy: strOr(p.CreatedByName), TillPack: strOr(p.TillPack), CreatedAt: p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// AdminProducts is the paged catalog: search (name/barcode, ranked), category
// (with its sub-categories), low stock, and disabled products on request.
func (h *handlers) AdminProducts(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	q := products.ListQuery{Search: strings.TrimSpace(c.QueryParam("q")), Page: page, Limit: productPageSize,
		LowStock: c.QueryParam("low") == "1", IncludeInactive: c.QueryParam("disabled") == "1"}
	if id, err := strconv.ParseInt(c.QueryParam("category"), 10, 64); err == nil && id > 0 {
		q.CategoryID = &id
	}
	if id, err := strconv.ParseInt(c.QueryParam("supplier"), 10, 64); err == nil && id > 0 {
		q.PreferredSupplierID = &id
	}
	rows, total, err := h.t.products.List(c.Request().Context(), q)
	if err != nil {
		return err
	}
	out := ProductList{Items: make([]AdminProductRow, 0, len(rows)), Total: total, Page: max(page, 1), PageSize: productPageSize}
	splits := h.stockSplits(c.Request().Context(), rows)
	for _, p := range rows {
		r := productRow(p)
		r.StockSplit = splits[p.ID]
		out.Items = append(out.Items, r)
	}
	if products.MetaProvider != nil && len(rows) > 0 {
		ids := make([]int64, len(rows))
		for i, p := range rows {
			ids[i] = p.ID
		}
		meta := products.MetaProvider(c.Request().Context(), ids)
		for i := range out.Items {
			out.Items[i].Meta = meta[out.Items[i].ID]
		}
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminProduct(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	p, err := h.t.products.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	var sup int64
	if p.PreferredSupplierID != nil {
		sup = *p.PreferredSupplierID
	}
	out := AdminProduct{AdminProductRow: productRow(*p), UnitID: p.UnitID, TaxRate: p.TaxRate.String(), WarrantyMonths: p.WarrantyMonths, SupplierID: sup,
		ReorderAuto: p.ReorderAuto, ReorderStart: p.ReorderStart, ReorderCalc: calcView(calcOf(*p))}
	out.FoldFrom = h.foldSource(c.Request().Context(), id)
	return c.JSON(http.StatusOK, out)
}

func bindProduct(c echo.Context) (products.CreateInput, ProductRequest, error) {
	var req ProductRequest
	if err := c.Bind(&req); err != nil {
		return products.CreateInput{}, req, apperr.BadRequest("invalid request body")
	}
	return bindProductReq(c, req)
}

// bindProductReq maps and validates a product form (blank money = 0).
func bindProductReq(c echo.Context, req ProductRequest) (products.CreateInput, ProductRequest, error) {
	orZero := func(s string) string {
		if s = strings.TrimSpace(s); s == "" {
			return "0"
		}
		return s
	}
	in := products.CreateInput{
		Name: strings.TrimSpace(req.Name), NameLocal: optStr(req.NameLocal), Barcode: optStr(req.Barcode), CategoryID: req.CategoryID, UnitID: req.UnitID,
		CostPrice: orZero(req.CostPrice), SellingPrice: orZero(req.SellingPrice), WholesalePrice: orZero(req.WholesalePrice), TaxRate: orZero(req.TaxRate),
		ReorderLevel: req.ReorderLevel, TrackSerial: req.TrackSerial, WarrantyMonths: req.WarrantyMonths, RepriceStock: req.RepriceStock,
		ReorderAuto: req.ReorderAuto, ReorderStart: req.ReorderStart,
	}
	if req.SupplierID > 0 {
		in.PreferredSupplierID = &req.SupplierID
	}
	if in.CategoryID <= 0 {
		return in, req, apperr.Validation("pick a category")
	}
	if in.UnitID <= 0 {
		return in, req, apperr.Validation("pick a unit")
	}
	return in, req, c.Validate(&in)
}

func (h *handlers) AdminProductCreate(c echo.Context) error {
	in, req, err := bindProduct(c)
	if err != nil {
		return err
	}
	// Duplicate guard (karots-pos): a copy must be renamed, or it becomes a same-named twin.
	if src := strings.TrimSpace(req.DupSourceName); src != "" && strings.EqualFold(src, in.Name) {
		return apperr.Validation("change the name before saving the copy")
	}
	ctx := c.Request().Context()
	x := plugin.ExtraCtx{CategoryID: in.CategoryID, IsNew: true}
	if err := checkExtras(ctx, req, x); err != nil {
		return err
	}
	p, err := h.t.products.Create(ctx, in)
	if err != nil {
		return err
	}
	x.ProductID = p.ID
	if err := saveExtras(ctx, req, x); err != nil {
		return err
	}
	h.relearn(c, p)
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), "created "+in.Name)
	return c.JSON(http.StatusCreated, ProductSaved{ID: p.ID, Message: "Product added"})
}

// AdminProductUpdate saves the form. Saving a quick-added item finishes it: the
// review flag clears and its past sale lines get the real cost (their COGS was 0).
func (h *handlers) AdminProductUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, req, err := bindProduct(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	x := plugin.ExtraCtx{ProductID: id, CategoryID: in.CategoryID}
	if err := checkExtras(ctx, req, x); err != nil {
		return err
	}
	p, err := h.t.products.Update(ctx, id, in)
	if err != nil {
		return err
	}
	h.relearn(c, p)
	if err := saveExtras(ctx, req, x); err != nil {
		return err
	}
	out := ProductSaved{ID: id, Message: "Product saved"}
	// The form shows its packs, so saving it counts as checking a till-set size.
	if err := h.t.products.ClearPacksSetAtTill(ctx, id); err != nil {
		return apperr.Internal("failed to update packs", err)
	}
	if !p.NeedsReview {
		h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "updated "+in.Name)
		return c.JSON(http.StatusOK, out)
	}
	if err := h.t.products.MarkReviewed(ctx, id); err != nil {
		return err
	}
	out.Message = "Item finished and off the review list"
	if n, err := h.t.products.BackfillCost(ctx, id, in.CostPrice); err == nil && n > 0 {
		out.Message = "Item finished — cost corrected on " + strconv.FormatInt(n, 10) + " past sale line(s)"
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "finalized quick-add "+in.Name)
	return c.JSON(http.StatusOK, out)
}

// AdminProductDisable soft-deletes: the product leaves the till and pickers but
// keeps its history, and can be re-enabled.
func (h *handlers) AdminProductDisable(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.products.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "product", strconv.FormatInt(id, 10), "disabled")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminProductReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.products.Reactivate(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "reactivated")
	return c.NoContent(http.StatusNoContent)
}

// AdminProductReview lists items quick-added at the till that still need their
// real category, unit, cost and count (karots-pos "Items to review").
func (h *handlers) AdminProductReview(c echo.Context) error {
	rows, err := h.t.products.NeedsReview(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]AdminProductRow, 0, len(rows))
	for _, p := range rows {
		out = append(out, productRow(p))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminProductReviewCount(c echo.Context) error {
	n, err := h.t.products.CountNeedsReview(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, Count{Count: n})
}

// AdminProductReviewDone clears the flag for an item that's fine as it is.
func (h *handlers) AdminProductReviewDone(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.products.MarkReviewed(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(id, 10), "marked reviewed")
	return c.NoContent(http.StatusNoContent)
}

// AdminSupplierOptions feeds the preferred-supplier picker (active suppliers).
func (h *handlers) AdminSupplierOptions(c echo.Context) error {
	rows, err := suppliers.NewService(h.d.DB).List(c.Request().Context(), "")
	if err != nil {
		return err
	}
	out := make([]Option, 0, len(rows))
	for _, s := range rows {
		out = append(out, Option{ID: s.ID, Name: s.Name})
	}
	return c.JSON(http.StatusOK, out)
}
