package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/heldsales"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerTillSales(g *echo.Group) {
	g.POST("/sales", h.TillCreateSale)
	g.POST("/sales/:id/print", h.TillPrintSale)
	g.GET("/held", h.TillHeld)
	g.POST("/held", h.TillHold)
	g.DELETE("/held/:id", h.TillDeleteHeld)
}

func saleInput(r SaleRequest) sales.CreateInput {
	in := sales.CreateInput{
		CustomerID: r.CustomerID, SaleType: r.SaleType, Discount: r.Discount, DiscountType: r.DiscountType,
		AllowOverLimit: r.AllowOverLimit, ChangeGiven: r.ChangeGiven, OnAccountRounding: r.OnAccountRounding,
		DeferDrawerKick: r.DeferDrawerKick,
		Items:           make([]sales.ItemInput, 0, len(r.Items)),
		Payments:        make([]sales.PaymentInput, 0, len(r.Payments)),
	}
	for _, it := range r.Items {
		comps := make([]sales.ServiceComponent, 0, len(it.Components))
		for _, c := range it.Components {
			comps = append(comps, sales.ServiceComponent{ProductID: c.ProductID, Quantity: c.Quantity})
		}
		item := sales.ItemInput{
			ProductID: it.ProductID, Quantity: it.Quantity, Discount: it.Discount, DiscountType: it.DiscountType,
			PriceOverride: it.PriceOverride, BatchID: it.BatchID, AllowOversell: it.AllowOversell,
			Serials: it.Serials, Description: it.Description, Components: comps, Payout: it.Payout,
		}
		for k, v := range it.Plugin {
			if b, err := json.Marshal(v); err == nil {
				if item.Plugin == nil {
					item.Plugin = map[string]json.RawMessage{}
				}
				item.Plugin[k] = b
			}
		}
		if g := it.Group; g != nil {
			sets, _ := decimal.NewFromString(g.Sets) // a bad value stays 0 and is refused as a validation error
			item.Group = &sales.LineGroup{No: g.No, Label: g.Label, Sets: sets}
		}
		in.Items = append(in.Items, item)
	}
	for _, p := range r.Payments {
		pi := sales.PaymentInput{Method: p.Method, Amount: p.Amount, AccountID: p.AccountID, Meta: p.Meta}
		if ref := strings.TrimSpace(p.Reference); ref != "" {
			pi.Reference = &ref
		}
		in.Payments = append(in.Payments, pi)
	}
	return in
}

func saleResult(d *sales.Detail) SaleResult {
	s := d.Sale
	out := SaleResult{ID: s.ID, ReceiptNo: s.ReceiptNo, Total: dec2(s.Total), PaidAmount: dec2(s.PaidAmount),
		ChangeGiven: dec2(s.ChangeGiven), RoundingToAccount: dec2(s.RoundingToAccount),
		CustomerName: strOr(s.CustomerName), Methods: []string{}}
	if s.CustomerBalanceAfter != nil {
		v := dec2(*s.CustomerBalanceAfter)
		out.CustomerBalanceAfter = &v
	}
	seen := map[string]bool{}
	for _, p := range d.Payments {
		if !seen[p.Method] {
			seen[p.Method] = true
			out.Methods = append(out.Methods, p.Method)
		}
	}
	return out
}

// TillCreateSale posts a sale. The server recomputes every amount; the over-limit
// approval is honoured only for a user the role/flag permits — never the client alone.
func (h *handlers) TillCreateSale(c echo.Context) error {
	var req SaleRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := saleInput(req)
	if err := c.Validate(&in); err != nil {
		return err
	}
	if in.AllowOverLimit && !middleware.MayManageCredit(middleware.CurrentRole(c), middleware.CanManageCredit(c)) {
		in.AllowOverLimit = false
	}
	detail, err := h.d.Sales.Create(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, saleResult(detail))
}

func heldDTO(x heldsales.HeldSale) HeldSale {
	return HeldSale{ID: x.ID, Label: strOr(x.Label), SaleType: x.SaleType, CustomerID: x.CustomerID,
		Discount: dec2(x.Discount), Cart: string(x.Cart), ItemCount: x.ItemCount, Total: dec2(x.Total),
		CreatedAt: x.CreatedAt.Format(time.RFC3339)}
}

func (h *handlers) TillHeld(c echo.Context) error {
	rows, err := h.t.held.List(c.Request().Context(), middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	out := make([]HeldSale, 0, len(rows))
	for _, r := range rows {
		out = append(out, heldDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) TillHold(c echo.Context) error {
	var req HoldRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if !json.Valid([]byte(req.Cart)) {
		return apperr.BadRequest("cart must be JSON")
	}
	in := heldsales.Input{Label: req.Label, SaleType: req.SaleType, CustomerID: req.CustomerID,
		Discount: req.Discount, Cart: json.RawMessage(req.Cart), ItemCount: req.ItemCount, Total: req.Total}
	if err := c.Validate(&in); err != nil {
		return err
	}
	held, err := h.t.held.Hold(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, heldDTO(*held))
}

func (h *handlers) TillDeleteHeld(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.held.Delete(c.Request().Context(), id, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
