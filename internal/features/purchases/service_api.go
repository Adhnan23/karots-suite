package purchases

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
	rows, err := h.svc.List(c.Request().Context())
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
	d, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.OK(c, d)
}

func (h *APIHandler) Create(c echo.Context) error {
	var in CreateInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	d, err := h.svc.Create(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return response.Created(c, d)
}

func RegisterAPI(e *echo.Echo, db *sqlx.DB, cfg *config.Config) {
	api := NewAPIHandler(NewService(db))
	g := e.Group("/api/purchases", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole("admin", "manager"))
	g.GET("", api.List)
	g.GET("/:id", api.Get)
	g.POST("", api.Create)
}
