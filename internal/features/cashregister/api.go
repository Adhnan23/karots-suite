// Package cashregister manages daily cash-drawer sessions. Exactly one session
// per cashier can be open at a time (enforced by a partial unique index).
//
// The drawer is counted by denomination (how many of each note/coin) at open and
// close. Every cash event during a session — opening float, mid-shift
// withdrawals and pay-ins, credit collected, and the close — is written to the
// cash_movements ledger so the whole day is auditable and rolls into finance.
//
// Expected cash at any moment = opening float + cash sales collected + pay-ins
// and credit collected − withdrawals/refunds. The close compares this to the
// counted drawer and records the over/short difference.
package cashregister

import (
	"encoding/json"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/response"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

type APIHandler struct{ svc *Service }

func NewAPIHandler(svc *Service) *APIHandler { return &APIHandler{svc: svc} }

func (h *APIHandler) Current(c echo.Context) error {
	sess, err := h.svc.Current(c.Request().Context(), middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return response.OK(c, sess)
}

func (h *APIHandler) Summary(c echo.Context) error {
	sum, err := h.svc.Summary(c.Request().Context(), middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return response.OK(c, sum)
}

func (h *APIHandler) Open(c echo.Context) error {
	var in OpenInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	sess, err := h.svc.Open(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return response.Created(c, sess)
}

func (h *APIHandler) Close(c echo.Context) error {
	var in CloseInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	res, err := h.svc.Close(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

func (h *APIHandler) Withdraw(c echo.Context) error {
	var in MovementInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sum, err := h.svc.Withdraw(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return response.OK(c, sum)
}

func (h *APIHandler) PayIn(c echo.Context) error {
	var in MovementInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sum, err := h.svc.PayIn(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return response.OK(c, sum)
}

func RegisterAPI(e *echo.Echo, db *sqlx.DB, cfg *config.Config, salesSvc *sales.Service, auditSvc *audit.Service) *Service {
	svc := NewService(db, salesSvc).WithAudit(auditSvc)
	api := NewAPIHandler(svc)
	g := e.Group("/api/cash-register", middleware.JWTAuth(cfg.JWTSecret))
	g.GET("/current", api.Current)
	g.GET("/summary", api.Summary)
	g.POST("/open", api.Open)
	g.POST("/close", api.Close)
	g.POST("/withdraw", api.Withdraw)
	g.POST("/pay-in", api.PayIn)
	return svc
}

// amountFrom resolves the drawer amount: prefer the denomination breakdown (the
// authoritative count) and fall back to a typed total. Returns the canonical
// breakdown JSON to persist (nil when none was supplied).
func amountFrom(typed string, breakdown json.RawMessage) (decimal.Decimal, []byte, bool) {
	if total, raw, ok := breakdownTotal(breakdown); ok {
		return total, raw, true
	}
	if strings.TrimSpace(typed) == "" {
		return decimal.Zero, nil, false
	}
	v, err := money.Parse(typed)
	if err != nil {
		return decimal.Zero, nil, false
	}
	return v, nil, true
}

// breakdownTotal sums value×qty over a {denominationValue: count} map.
func breakdownTotal(raw json.RawMessage) (decimal.Decimal, []byte, bool) {
	if len(raw) == 0 {
		return decimal.Zero, nil, false
	}
	m := map[string]int{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return decimal.Zero, nil, false
	}
	total := decimal.Zero
	any := false
	clean := map[string]int{}
	for k, qty := range m {
		if qty <= 0 {
			continue
		}
		v, err := decimal.NewFromString(k)
		if err != nil || !v.IsPositive() {
			continue
		}
		total = total.Add(v.Mul(decimal.NewFromInt(int64(qty))))
		clean[k] = qty
		any = true
	}
	if !any {
		// An all-zero count is still a valid (empty) drawer.
		return decimal.Zero, []byte("{}"), true
	}
	out, _ := json.Marshal(clean)
	return total, out, true
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
