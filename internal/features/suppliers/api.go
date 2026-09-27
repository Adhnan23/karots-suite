// Package suppliers manages wholesale suppliers and their outstanding payables.
package suppliers

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

func (h *APIHandler) Create(c echo.Context) error {
	var in CreateInput
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sup, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	return response.Created(c, sup)
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

func (h *APIHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperr.BadRequest("invalid id")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return response.NoContent(c)
}

func RegisterAPI(e *echo.Echo, db *sqlx.DB, cfg *config.Config) {
	api := NewAPIHandler(NewService(db))
	g := e.Group("/api/suppliers", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole("admin", "manager"))
	g.GET("", api.List)
	g.POST("", api.Create)
	g.PUT("/:id", api.Update)
	g.DELETE("/:id", api.Delete)
}
