package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const purchasePageSize = 30

// registerAdminPurchases mounts purchase orders → receive (karots-pos
// /admin/purchases). Static paths come before /:id.
func (h *handlers) registerAdminPurchases(g *echo.Group) {
	g.GET("/purchases", h.AdminPurchases)
	g.POST("/purchases", h.AdminPurchaseOrder)
	g.POST("/purchases/receive", h.AdminPurchaseReceiveNow)
	g.GET("/purchases/print", h.AdminPurchasesPrint)
	g.GET("/purchases/:id", h.AdminPurchase)
	g.PUT("/purchases/:id", h.AdminPurchaseUpdate)
	g.DELETE("/purchases/:id", h.AdminPurchaseDelete)
	g.POST("/purchases/:id/receive", h.AdminPurchaseReceive)
}

func ymd(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func purchaseRow(p purchases.Purchase) PurchaseRow {
	return PurchaseRow{ID: p.ID, SupplierID: p.SupplierID, SupplierName: p.SupplierName, InvoiceNo: strOr(p.InvoiceNo), Status: p.Status,
		Date: p.CreatedAt.Format(time.RFC3339), ExpectedDate: ymd(p.ExpectedDate), DueDate: ymd(p.DueDate), Total: dec2(p.Total),
		Paid: dec2(p.PaidAmount), Credited: dec2(p.CreditedAmount), Balance: dec2(p.Balance()), By: strOr(p.ReceivedByName)}
}

// AdminPurchases lists orders (tab=orders) or received deliveries: supplier,
// search (invoice no / supplier), unpaid=1.
func (h *handlers) AdminPurchases(c echo.Context) error {
	sup, _ := strconv.ParseInt(c.QueryParam("supplier"), 10, 64)
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	rows, total, err := purchases.NewService(h.d.DB).Find(c.Request().Context(), purchases.Filter{Draft: c.QueryParam("tab") == "orders", SupplierID: sup,
		Q: c.QueryParam("q"), Unpaid: c.QueryParam("unpaid") == "1", Page: page, Limit: purchasePageSize})
	if err != nil {
		return err
	}
	out := PurchaseList{Items: make([]PurchaseRow, 0, len(rows)), Total: total, Page: page, PageSize: purchasePageSize}
	for _, p := range rows {
		out.Items = append(out.Items, purchaseRow(p))
	}
	return c.JSON(http.StatusOK, out)
}

// purchaseDetail shapes a purchase with its supplier block and each line's
// current product cost / price (for the receive screen's margin guard).
func (h *handlers) purchaseDetail(ctx context.Context, d *purchases.Detail) (PurchaseDetail, error) {
	p := d.Purchase
	out := PurchaseDetail{PurchaseRow: purchaseRow(p), Subtotal: dec2(p.Subtotal), Discount: dec2(p.Discount), DiscountType: p.DiscountType,
		DiscountVal: p.DiscountValue.String(), Notes: strOr(p.Notes), Lines: make([]PurchaseLine, 0, len(d.Items))}
	if p.DiscountValue.IsZero() {
		out.DiscountVal = ""
	}
	sup, err := suppliers.NewService(h.d.DB).Get(ctx, p.SupplierID)
	if err != nil {
		return out, err
	}
	out.Supplier = PurchaseParty{Name: sup.Name, Contact: strOr(sup.ContactPerson), Phone: strOr(sup.Phone), Address: strOr(sup.Address)}
	ids := make([]int64, 0, len(d.Items))
	for _, it := range d.Items {
		ids = append(ids, it.ProductID)
	}
	prods, err := h.t.products.ByIDs(ctx, ids)
	if err != nil {
		return out, err
	}
	type cur struct{ cost, sell, unit string }
	now := map[int64]cur{}
	for _, pr := range prods {
		now[pr.ID] = cur{dec2(pr.CostPrice), dec2(pr.SellingPrice), pr.UnitAbbr}
	}
	blank := func(v decimal.Decimal) string {
		if v.IsZero() {
			return ""
		}
		return v.String()
	}
	for _, it := range d.Items {
		ordered := ""
		if it.OrderedQty != nil {
			ordered = it.OrderedQty.String()
		} else if p.Status == "draft" {
			ordered = it.Quantity.String()
		}
		dt := it.DiscountType
		if dt == "" {
			dt = "fixed"
		}
		out.Lines = append(out.Lines, PurchaseLine{StockLine: StockLine{ProductID: it.ProductID, Name: it.ProductName, Ordered: ordered, Quantity: it.Quantity.String(),
			FreeQty: blank(it.FreeQty), CostPrice: it.CostPrice.String(), SellingPrice: blank(it.SellingPrice), ExpiryDate: ymd(it.ExpiryDate),
			Discount: blank(it.DiscountValue), DiscountType: dt}, Net: dec2(it.Subtotal), CurCost: now[it.ProductID].cost, CurSell: now[it.ProductID].sell, UnitAbbr: now[it.ProductID].unit})
	}
	return out, nil
}

func (h *handlers) AdminPurchase(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	d, err := purchases.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	out, err := h.purchaseDetail(ctx, d)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

// AdminPurchasesPrint loads several orders for one print (?ids=1,2,3).
func (h *handlers) AdminPurchasesPrint(c echo.Context) error {
	var ids []int64
	for _, raw := range strings.Split(c.QueryParam("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > 50 {
		return apperr.Validation("pick 1 to 50 purchase orders to print")
	}
	ctx := c.Request().Context()
	ds, err := purchases.NewService(h.d.DB).GetMany(ctx, ids)
	if err != nil {
		return err
	}
	out := make([]PurchaseDetail, 0, len(ds))
	for i := range ds {
		pd, err := h.purchaseDetail(ctx, &ds[i])
		if err != nil {
			return err
		}
		out = append(out, pd)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) orderInput(c echo.Context) (purchases.CreateInput, PurchaseRequest, error) {
	var req PurchaseRequest
	if err := c.Bind(&req); err != nil {
		return purchases.CreateInput{}, req, apperr.BadRequest("invalid request body")
	}
	items, err := h.itemInputs(c.Request().Context(), req.Lines)
	if err != nil {
		return purchases.CreateInput{}, req, err
	}
	in := purchases.CreateInput{SupplierID: req.SupplierID, InvoiceNo: optStr(req.InvoiceNo), Notes: optStr(req.Notes), Items: items,
		Discount: strings.TrimSpace(req.Discount), DiscountType: req.DiscountType, DueDate: strings.TrimSpace(req.DueDate), ExpectedDate: strings.TrimSpace(req.ExpectedDate)}
	return in, req, c.Validate(&in)
}

// AdminPurchaseOrder saves a purchase order (a draft: no stock, no payable).
func (h *handlers) AdminPurchaseOrder(c echo.Context) error {
	in, _, err := h.orderInput(c)
	if err != nil {
		return err
	}
	d, err := purchases.NewService(h.d.DB).CreateDraft(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase", strconv.FormatInt(d.Purchase.ID, 10), "purchase order for "+d.Purchase.SupplierName)
	return c.JSON(http.StatusCreated, purchaseRow(d.Purchase))
}

func (h *handlers) AdminPurchaseUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, _, err := h.orderInput(c)
	if err != nil {
		return err
	}
	d, err := purchases.NewService(h.d.DB).UpdateDraft(c.Request().Context(), id, in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "purchase", strconv.FormatInt(id, 10), "edited purchase order")
	return c.JSON(http.StatusOK, purchaseRow(d.Purchase))
}

// AdminPurchaseDelete drops a draft; a received purchase can't be (use a return).
func (h *handlers) AdminPurchaseDelete(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := purchases.NewService(h.d.DB).DeleteDraft(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "purchase", strconv.FormatInt(id, 10), "deleted purchase order")
	return c.NoContent(http.StatusNoContent)
}

// adminPayOnDelivery pays on delivery from an admin money source, clamped to
// what's left once supplier credit settled part of the bill.
func (h *handlers) adminPayOnDelivery(ctx context.Context, tx *sqlx.Tx, d *purchases.Detail, pay AdminPayNow, userID int64) (decimal.Decimal, *cashflow.Receipt, cashflow.Location, error) {
	amt, err := optAmount(pay.Amount, "payment amount")
	if err != nil || !amt.IsPositive() {
		return decimal.Zero, nil, cashflow.External(), err
	}
	method, ok := normSupplierMethod(pay.Method)
	if !ok {
		return amt, nil, cashflow.External(), apperr.Validation("invalid payment method")
	}
	src, err := adminMoneyLoc(method, pay.Source)
	if err != nil {
		return amt, nil, src, err
	}
	return h.settleDelivery(ctx, tx, d, amt, method, src, userID)
}

// receiveDone answers a receive: the purchase, what was paid, the slip.
func (h *handlers) receiveDone(c echo.Context, status int, d *purchases.Detail, paid decimal.Decimal, rec *cashflow.Receipt, src cashflow.Location) error {
	out := ReceiveResult{PurchaseID: d.Purchase.ID, Total: dec2(d.Purchase.Total), Paid: dec2(paid)}
	if rec != nil {
		out.ReceiptID = &rec.ID
		if src.Kind == cashflow.KindTill {
			h.d.KickDrawer(c.Request().Context())
		}
	}
	return c.JSON(status, out)
}

// AdminPurchaseReceiveNow books a delivery that came without an order (GRN):
// stock, lots and the payable land together, optionally paid on the spot.
func (h *handlers) AdminPurchaseReceiveNow(c echo.Context) error {
	in, req, err := h.orderInput(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	userID := middleware.CurrentUserID(c)
	var d *purchases.Detail
	var rec *cashflow.Receipt
	var src cashflow.Location
	paid := decimal.Zero
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if d, err = purchases.CreateTx(ctx, tx, in, userID); err != nil {
			return err
		}
		paid, rec, src, err = h.adminPayOnDelivery(ctx, tx, d, req.Pay, userID)
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase", strconv.FormatInt(d.Purchase.ID, 10), "received delivery (no prior order) from "+d.Purchase.SupplierName)
	return h.receiveDone(c, http.StatusCreated, d, paid, rec, src)
}

// AdminPurchaseReceive receives an order: what came (over or short), lines that
// weren't on it, and optionally the unreceived rest kept on a new order.
func (h *handlers) AdminPurchaseReceive(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, req, err := h.orderInput(c)
	if err != nil {
		return err
	}
	rin := purchases.ReceiveInput{InvoiceNo: in.InvoiceNo, Discount: in.Discount, DiscountType: in.DiscountType, DueDate: in.DueDate,
		Notes: in.Notes, Items: in.Items, KeepRemainder: req.KeepRemainder}
	ctx := c.Request().Context()
	userID := middleware.CurrentUserID(c)
	var d *purchases.Detail
	var rec *cashflow.Receipt
	var src cashflow.Location
	paid := decimal.Zero
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if d, err = purchases.ReceiveTx(ctx, tx, id, rin, userID); err != nil {
			return err
		}
		paid, rec, src, err = h.adminPayOnDelivery(ctx, tx, d, req.Pay, userID)
		return err
	})
	if err != nil {
		return err
	}
	detail := "received purchase order"
	if paid.IsPositive() {
		detail += ", paid " + money.Display(paid)
	}
	h.audit(c, audit.ActionUpdate, "purchase", strconv.FormatInt(id, 10), detail)
	return h.receiveDone(c, http.StatusOK, d, paid, rec, src)
}
