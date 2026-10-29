package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/purchasereturns"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const returnPageSize = 30

// registerAdminPurchaseReturns mounts debit notes.
// A posted return is read-only: it already moved stock and the payable, so a
// mistake is corrected with an opposing entry.
func (h *handlers) registerAdminPurchaseReturns(g *echo.Group) {
	g.GET("/purchase-returns", h.AdminPurchaseReturns)
	g.POST("/purchase-returns", h.AdminPurchaseReturnCreate)
	g.GET("/purchase-returns/:id", h.AdminPurchaseReturn)
	g.GET("/purchases/:id/returnable", h.AdminPurchaseReturnable)
}

// AdminPurchaseReturnable is what can still go back from one received delivery:
// each line's lot, what's left of it and what those units really cost.
func (h *handlers) AdminPurchaseReturnable(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	d, err := purchases.NewService(h.d.DB).Get(ctx, id)
	if err != nil {
		return err
	}
	if d.Purchase.Status == "draft" {
		return apperr.Validation("an order that hasn't come in has nothing to return")
	}
	type lot struct {
		ItemID int64           `db:"purchase_item_id"`
		ID     int64           `db:"id"`
		Got    decimal.Decimal `db:"qty_received"`
		Left   decimal.Decimal `db:"qty_remaining"`
		Cost   decimal.Decimal `db:"cost_price"`
		Expiry *time.Time      `db:"expiry_date"`
		Unit   string          `db:"unit_abbr"`
	}
	var lots []lot
	if err := h.d.DB.SelectContext(ctx, &lots, `
		SELECT b.purchase_item_id, b.id, b.qty_received, b.qty_remaining, b.cost_price, b.expiry_date, COALESCE(u.abbreviation, '') AS unit_abbr
		FROM stock_batches b JOIN purchase_items pi ON pi.id = b.purchase_item_id
		JOIN products p ON p.id = b.product_id LEFT JOIN units u ON u.id = p.unit_id
		WHERE pi.purchase_id = $1`, id); err != nil {
		return apperr.Internal("failed to load the delivery's lots", err)
	}
	byItem := map[int64]lot{}
	for _, l := range lots {
		byItem[l.ItemID] = l
	}
	out := ReturnableList{Lines: []ReturnableLine{}}
	for _, it := range d.Items {
		l, ok := byItem[it.ID]
		if !ok {
			continue
		}
		out.Lines = append(out.Lines, ReturnableLine{ProductID: it.ProductID, Name: it.ProductName, UnitAbbr: l.Unit, BatchID: l.ID,
			Received: l.Got.String(), Left: l.Left.String(), Cost: dec2(l.Cost), ListCost: dec2(it.CostPrice), Expiry: ymd(l.Expiry)})
	}
	return c.JSON(http.StatusOK, out)
}

func purchaseReturnRow(r purchasereturns.PurchaseReturn) PurchaseReturnRow {
	out := PurchaseReturnRow{ID: r.ID, SupplierID: r.SupplierID, SupplierName: r.SupplierName, InvoiceNo: strOr(r.InvoiceNo), Reference: strOr(r.Reference),
		Reason: strOr(r.Reason), Total: dec2(r.Total), Date: r.CreatedAt.Format(time.RFC3339)}
	if r.PurchaseID != nil {
		out.PurchaseID = *r.PurchaseID
	}
	return out
}

// AdminPurchaseReturns lists debit notes, newest first; supplier= narrows it.
func (h *handlers) AdminPurchaseReturns(c echo.Context) error {
	rows, err := purchasereturns.NewService(h.d.DB).List(c.Request().Context())
	if err != nil {
		return err
	}
	sup, _ := strconv.ParseInt(c.QueryParam("supplier"), 10, 64)
	kept := make([]PurchaseReturnRow, 0, len(rows))
	for _, r := range rows {
		if sup == 0 || r.SupplierID == sup {
			kept = append(kept, purchaseReturnRow(r))
		}
	}
	page := max(1, atoiOr(c.QueryParam("page"), 1))
	return c.JSON(http.StatusOK, PurchaseReturnList{Items: kept[min((page-1)*returnPageSize, len(kept)):min(page*returnPageSize, len(kept))],
		Total: len(kept), Page: page, PageSize: returnPageSize})
}

func (h *handlers) AdminPurchaseReturn(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	d, err := purchasereturns.NewService(h.d.DB).Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	out := PurchaseReturnDetail{PurchaseReturnRow: purchaseReturnRow(d.Return), Lines: make([]PurchaseReturnLine, 0, len(d.Items))}
	for _, it := range d.Items {
		l := PurchaseReturnLine{ProductID: it.ProductID, Name: it.ProductName, Quantity: it.Quantity.String(), CostPrice: dec2(it.CostPrice), Subtotal: dec2(it.Subtotal)}
		if it.BatchID != nil {
			l.BatchID = *it.BatchID
		}
		out.Lines = append(out.Lines, l)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminPurchaseReturnCreate posts a debit note: the named lots leave stock and
// the value credits the supplier's open bills oldest first.
func (h *handlers) AdminPurchaseReturnCreate(c echo.Context) error {
	var req PurchaseReturnRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := purchasereturns.CreateInput{SupplierID: req.SupplierID, PurchaseID: req.PurchaseID, Reference: optStr(req.Reference), Reason: optStr(req.Reason)}
	for _, l := range req.Lines {
		in.Items = append(in.Items, purchasereturns.ItemInput{ProductID: l.ProductID, Quantity: strings.TrimSpace(l.Quantity), CostPrice: strings.TrimSpace(l.CostPrice), BatchID: l.BatchID})
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	svc := purchasereturns.NewService(h.d.DB)
	made, err := svc.Create(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	d, err := svc.Get(c.Request().Context(), made.Return.ID) // Create's row has no supplier name / date
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "purchase_return", strconv.FormatInt(d.Return.ID, 10), "returned "+money.Display(d.Return.Total)+" of stock to "+d.Return.SupplierName)
	return c.JSON(http.StatusCreated, purchaseReturnRow(d.Return))
}
