// Package customers manages registered customers, especially credit ("vade")
// buyers. Walk-in sales leave customer_id NULL.
package customers

import (
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/middleware"
	"karots-suite/internal/response"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type APIHandler struct{ svc *Service }

func NewAPIHandler(svc *Service) *APIHandler { return &APIHandler{svc: svc} }

func (h *APIHandler) List(c echo.Context) error {
	rows, err := h.svc.List(c.Request().Context(), c.QueryParam("search"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *APIHandler) Get(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid id")
	}
	cust, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.OK(c, cust)
}

func (h *APIHandler) Create(c echo.Context) error {
	var in CreateInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	// The till (cashier) may never seed an opening / old-debt balance — that is an
	// admin-only figure. Force it to zero regardless of what the client sent.
	in.OpeningBalance = ""
	cust, existed, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	// 200 when we reused an existing customer (phone matched), 201 when a new one
	// was created — the till uses this to tell the cashier "already exists".
	if existed {
		return response.OK(c, cust)
	}
	return response.Created(c, cust)
}

func (h *APIHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid id")
	}
	var in UpdateInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.svc.Update(c.Request().Context(), id, in); err != nil {
		return err
	}
	return response.NoContent(c)
}

// CreditLimitInput is the narrow till payload for an inline limit change.
type CreditLimitInput struct {
	CreditLimit string `json:"credit_limit" validate:"required"`
}

// SetLimit updates only a customer's credit limit — the till's inline raise.
// Gated by RequireManageCredit; the admin PUT stays admin/manager-only.
func (h *APIHandler) SetLimit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid id")
	}
	var in CreditLimitInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.svc.SetCreditLimit(c.Request().Context(), id, in.CreditLimit); err != nil {
		return err
	}
	return response.NoContent(c)
}

func (h *APIHandler) Payment(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid id")
	}
	var in PaymentInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.svc.RecordPayment(c.Request().Context(), id, in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return response.NoContent(c)
}

func RegisterAPI(e *echo.Echo, db *sqlx.DB, cfg *config.Config) {
	api := NewAPIHandler(NewService(db))
	g := e.Group("/api/customers", middleware.JWTAuth(cfg.JWTSecret))
	g.GET("", api.List)
	g.GET("/:id", api.Get)
	g.POST("", api.Create, middleware.RequireRole("admin", "manager", "cashier"))
	g.PUT("/:id", api.Update, middleware.RequireRole("admin", "manager"))
	g.PATCH("/:id/credit-limit", api.SetLimit, middleware.RequireManageCredit())
	g.POST("/:id/payment", api.Payment, middleware.RequireRole("admin", "manager", "cashier"))
}
