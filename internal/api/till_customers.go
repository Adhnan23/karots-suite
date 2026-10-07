package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerTillCustomers(g *echo.Group) {
	g.GET("/customers", h.TillCustomers)
	g.POST("/customers", h.TillCreateCustomer)
	g.PATCH("/customers/:id/credit-limit", h.TillSetCreditLimit, middleware.RequireManageCredit())
	g.GET("/barcode/generate", h.TillGenerateBarcode)
	g.POST("/quick-item", h.TillQuickItem)
	g.POST("/write-off", h.TillWriteOff)
}

func tillCustomer(c customers.Customer) TillCustomer {
	return TillCustomer{ID: c.ID, Name: c.Name, Phone: strOr(c.Phone),
		CreditLimit: dec2(c.CreditLimit), Outstanding: dec2(c.OutstandingBalance)}
}

// TillCustomers returns every active customer; the till filters client-side
// (name/phone, top 50), as karots-pos did. Every one: List stops at 100 by
// name, so customer 101 could not be picked at the till.
func (h *handlers) TillCustomers(c echo.Context) error {
	rows, err := h.t.customers.AllActive(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]TillCustomer, 0, len(rows))
	for _, r := range rows {
		out = append(out, tillCustomer(r))
	}
	return c.JSON(http.StatusOK, out)
}

// TillCreateCustomer adds a customer from the till. Phone is required — it is
// the key that stops duplicate credit accounts; a matching phone returns the
// existing customer (200, existed) instead of creating a second one.
func (h *handlers) TillCreateCustomer(c echo.Context) error {
	var req CustomerCreateRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	phone := strings.TrimSpace(req.Phone)
	in := customers.CreateInput{Name: strings.TrimSpace(req.Name), Phone: &phone, CreditLimit: req.CreditLimit}
	if err := c.Validate(&in); err != nil {
		return err
	}
	cust, existed, err := h.t.customers.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	return c.JSON(status, CustomerCreated{Customer: tillCustomer(*cust), Existed: existed})
}

func (h *handlers) TillSetCreditLimit(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req CreditLimitRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.CreditLimit) == "" {
		return apperr.Validation("enter a credit limit")
	}
	if err := h.t.customers.SetCreditLimit(c.Request().Context(), id, req.CreditLimit); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "customer", strconv.FormatInt(id, 10), "credit limit set at till: "+req.CreditLimit)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) TillGenerateBarcode(c echo.Context) error {
	code, err := h.t.products.GenerateBarcode(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, GeneratedBarcode{Barcode: code})
}

// TillQuickItem sells something not yet in the catalogue: a minimal product with
// its stock seeded, flagged for admin review (products.QuickCreate).
func (h *handlers) TillQuickItem(c echo.Context) error {
	var req QuickItemRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	p, err := h.t.products.QuickCreate(c.Request().Context(), products.QuickInput{
		Name: req.Name, Price: req.Price, Qty: req.Qty, Barcode: req.Barcode, UnitID: req.UnitID,
	}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), "quick-add at till: "+p.Name)
	return c.JSON(http.StatusOK, tillProduct(*p))
}

// writeOffMsg is the audit line per reason (karots-pos consumeMsg).
var writeOffMsg = map[string]string{"damage": "wrote off at till", "own_use": "shop own use at till", "staff": "staff welfare at till"}

// TillWriteOff removes stock for a non-sale reason (damage by default; the
// Write-off tab also books own use / staff welfare), batch-specific so the till
// stops offering that lot. Same permission as karots-pos's cashier damage route.
func (h *handlers) TillWriteOff(c echo.Context) error {
	var req WriteOffRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if req.Reason == "" {
		req.Reason = "damage"
	}
	in := stock.ConsumeInput{ProductID: req.ProductID, Quantity: req.Quantity, Reason: req.Reason, Note: req.Note, BatchID: req.BatchID}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.t.stock.Consume(c.Request().Context(), in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(req.ProductID, 10), writeOffMsg[req.Reason]+": "+req.Note)
	return c.NoContent(http.StatusNoContent)
}
