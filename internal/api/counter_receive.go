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
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// itemInputs maps counter lines to purchase lines. A blank cost (an order taken
// without prices) falls back to the product's current cost.
func (h *handlers) itemInputs(ctx context.Context, lines []StockLine) ([]purchases.ItemInput, error) {
	out := make([]purchases.ItemInput, 0, len(lines))
	for _, l := range lines {
		cost := strings.TrimSpace(l.CostPrice)
		if cost == "" {
			p, err := h.t.products.Get(ctx, l.ProductID)
			if err != nil {
				return nil, err
			}
			cost = p.CostPrice.String()
		}
		out = append(out, purchases.ItemInput{ProductID: l.ProductID, Quantity: l.Quantity, FreeQty: l.FreeQty, CostPrice: cost,
			SellingPrice: l.SellingPrice, ExpiryDate: l.ExpiryDate, OrderedQty: l.Ordered, Discount: l.Discount, DiscountType: l.DiscountType})
	}
	return out, nil
}

func optStr(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// payOnDelivery settles up to what's still owed on the new purchase, in the same
// tx as the stock landing (karots-pos ReceiveWalkIn / ReceiveAgainstOrder).
func (h *handlers) payOnDelivery(ctx context.Context, tx *sqlx.Tx, c echo.Context, pur *purchases.Detail, pay PayNow, userID int64) (decimal.Decimal, *cashflow.Receipt, cashflow.Location, error) {
	src := cashflow.External()
	amt := decimal.Zero
	if raw := strings.TrimSpace(pay.Amount); raw != "" {
		v, err := money.Parse(raw)
		if err != nil || v.IsNegative() {
			return amt, nil, src, apperr.Validation("payment amount must be a non-negative number")
		}
		amt = v
	}
	if !amt.IsPositive() {
		return decimal.Zero, nil, src, nil
	}
	method, ok := normSupplierMethod(pay.Method)
	if !ok {
		return amt, nil, src, apperr.Validation("invalid payment method")
	}
	src, err := h.supplierMoneyLoc(c, method, pay.LockerID)
	if err != nil {
		return amt, nil, src, err
	}
	return h.settleDelivery(ctx, tx, pur, amt, method, src, userID)
}

// settleDelivery pays up to what's still owed on a just-booked purchase (credit
// settlement may already have covered some of it) from src.
func (h *handlers) settleDelivery(ctx context.Context, tx *sqlx.Tx, pur *purchases.Detail, amt decimal.Decimal, method string, src cashflow.Location, userID int64) (decimal.Decimal, *cashflow.Receipt, cashflow.Location, error) {
	fresh, err := purchases.NewRepository(tx).FindByID(ctx, pur.Purchase.ID)
	if err != nil {
		return amt, nil, src, apperr.Internal("failed to re-read the invoice", err)
	}
	if bal := fresh.Balance(); amt.GreaterThan(bal) {
		amt = decimal.Max(bal, decimal.Zero)
	}
	if !amt.IsPositive() {
		return decimal.Zero, nil, src, nil
	}
	rec, err := h.paySupplierTx(ctx, tx, pur.Purchase.SupplierID, pur.Purchase.SupplierName,
		supplierpay.PayInput{Method: method, Allocations: []supplierpay.Alloc{{PurchaseID: pur.Purchase.ID, Amount: amt}}}, src, userID)
	return amt, rec, src, err
}

func (h *handlers) receiveResult(c echo.Context, status int, d *purchases.Detail, paid decimal.Decimal, rec *cashflow.Receipt, src cashflow.Location, deferKick bool) error {
	out := ReceiveResult{PurchaseID: d.Purchase.ID, Total: dec2(d.Purchase.Total), Paid: dec2(paid)}
	if rec != nil {
		out.ReceiptID = &rec.ID
		if src.Kind == cashflow.KindTill && !deferKick {
			h.d.KickDrawer(c.Request().Context())
		}
	}
	return c.JSON(status, out)
}

// CounterReceiveWalkIn books a delivery that arrived without an order.
func (h *handlers) CounterReceiveWalkIn(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ReceiveRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	items, err := h.itemInputs(ctx, req.Lines)
	if err != nil {
		return err
	}
	in := purchases.CreateInput{SupplierID: id, InvoiceNo: optStr(req.InvoiceNo), Notes: optStr(req.Notes), Items: items,
		Discount: req.Discount, DiscountType: req.DiscountType}
	if err := c.Validate(&in); err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	var d *purchases.Detail
	var rec *cashflow.Receipt
	var src cashflow.Location
	paid := decimal.Zero
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if d, err = purchases.CreateTx(ctx, tx, in, userID); err != nil {
			return err
		}
		paid, rec, src, err = h.payOnDelivery(ctx, tx, c, d, req.Pay, userID)
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase", strconv.FormatInt(d.Purchase.ID, 10), "received a delivery from "+d.Purchase.SupplierName+" at the counter")
	return h.receiveResult(c, http.StatusCreated, d, paid, rec, src, req.DeferDrawerKick)
}

// CounterReceiveOrder receives goods against a draft order.
func (h *handlers) CounterReceiveOrder(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req ReceiveRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	items, err := h.itemInputs(ctx, req.Lines)
	if err != nil {
		return err
	}
	in := purchases.ReceiveInput{InvoiceNo: optStr(req.InvoiceNo), Notes: optStr(req.Notes), Items: items, KeepRemainder: req.KeepRemainder,
		Discount: req.Discount, DiscountType: req.DiscountType}
	if err := c.Validate(&in); err != nil {
		return err
	}
	userID := middleware.CurrentUserID(c)
	var d *purchases.Detail
	var rec *cashflow.Receipt
	var src cashflow.Location
	paid := decimal.Zero
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		if d, err = purchases.ReceiveTx(ctx, tx, id, in, userID); err != nil {
			return err
		}
		paid, rec, src, err = h.payOnDelivery(ctx, tx, c, d, req.Pay, userID)
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "purchase", strconv.FormatInt(id, 10), "received an order at the counter")
	return h.receiveResult(c, http.StatusOK, d, paid, rec, src, req.DeferDrawerKick)
}

// CounterOrder loads a draft order for receiving (or printing).
func (h *handlers) CounterOrder(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	d, err := purchases.NewService(h.d.DB).Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	v := OrderView{ID: d.Purchase.ID, SupplierID: d.Purchase.SupplierID, SupplierName: d.Purchase.SupplierName,
		Date: d.Purchase.CreatedAt.Format(time.RFC3339), Status: d.Purchase.Status, Notes: strOr(d.Purchase.Notes), Lines: []StockLine{}}
	for _, it := range d.Items {
		ordered := it.Quantity
		if it.OrderedQty != nil {
			ordered = *it.OrderedQty
		}
		sell := "" // 0 = no own price: the lot keeps the product's (shown as "same")
		if it.SellingPrice.IsPositive() {
			sell = it.SellingPrice.String()
		}
		v.Lines = append(v.Lines, StockLine{ProductID: it.ProductID, Name: it.ProductName, Ordered: ordered.String(), Quantity: ordered.String(),
			CostPrice: it.CostPrice.String(), SellingPrice: sell})
	}
	return c.JSON(http.StatusOK, v)
}

// CounterOrderCreate takes a supplier order (a draft purchase) at the counter.
func (h *handlers) CounterOrderCreate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req OrderRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	ctx := c.Request().Context()
	items, err := h.itemInputs(ctx, req.Lines)
	if err != nil {
		return err
	}
	in := purchases.CreateInput{SupplierID: id, Notes: optStr(req.Notes), Items: items, ExpectedDate: strings.TrimSpace(req.WantedBy)}
	if err := c.Validate(&in); err != nil {
		return err
	}
	d, err := purchases.NewService(h.d.DB).CreateDraft(ctx, in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase", strconv.FormatInt(d.Purchase.ID, 10), "took a supplier order at the counter")
	return c.JSON(http.StatusCreated, SupplierOrder{ID: d.Purchase.ID, Date: d.Purchase.CreatedAt.Format(time.RFC3339), Total: dec2(d.Purchase.Total)})
}

// CounterSupplierCreate adds a supplier met at the counter (name + phone only).
func (h *handlers) CounterSupplierCreate(c echo.Context) error {
	var req NewSupplierRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := suppliers.CreateInput{Name: strings.TrimSpace(req.Name), Phone: optStr(req.Phone), OpeningBalance: "0"}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sup, err := suppliers.NewService(h.d.DB).Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "supplier", strconv.FormatInt(sup.ID, 10), "added supplier at the counter: "+sup.Name)
	return c.JSON(http.StatusCreated, supplierRow(*sup))
}

// CounterIntakeProduct creates a product first met on a delivery or an order.
func (h *handlers) CounterIntakeProduct(c echo.Context) error {
	var req IntakeProductRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := products.IntakeInput{Name: req.Name, Barcode: req.Barcode, Cost: req.CostPrice, Selling: req.SellingPrice, UnitID: req.UnitID}
	note := "added on a delivery at the counter: "
	if req.Wanted {
		in.Cost, in.Selling = "", "" // never take a quoted price as fact
		note = "added while taking an order at the counter: "
	} else {
		for _, f := range []struct{ v, label string }{{req.CostPrice, "cost price"}, {req.SellingPrice, "selling price"}} {
			if v, err := money.Parse(strings.TrimSpace(f.v)); err != nil || !v.IsPositive() {
				return apperr.Validation(f.label + " is required")
			}
		}
	}
	p, err := h.t.products.CreateForIntake(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), note+p.Name)
	return c.JSON(http.StatusCreated, tillProduct(*p))
}
