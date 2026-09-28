// Package api is karots-suite's JSON HTTP layer (/api/v1). Handlers are thin:
// bind → call a domain service → return a DTO from dto.go. One file per area;
// every protected area group uses h.protected.
package api

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type Deps struct {
	DB           *sqlx.DB
	Cfg          *config.Config
	Auth         *auth.Service
	Settings     *settings.Service
	CashRegister *cashregister.Service
	KickDrawer   func(context.Context)
}

type handlers struct {
	d         Deps
	protected []echo.MiddlewareFunc // jwt → unlocked → PIN chosen
	t         tillServices
}

func Register(e *echo.Echo, d Deps) {
	jwt := middleware.JWTAuth(d.Cfg.JWTSecret)
	limiter := auth.NewLoginLimiter()
	h := &handlers{d: d, protected: []echo.MiddlewareFunc{jwt, middleware.RequireUnlocked(), middleware.RequirePinChosen()}, t: newTillServices(d.DB)}

	v1 := e.Group("/api/v1")
	v1.GET("/auth/login-info", h.LoginInfo)
	v1.POST("/auth/login", h.Login, limiter)
	v1.POST("/auth/logout", h.Logout)
	v1.POST("/auth/lock", h.Lock, jwt)
	v1.POST("/auth/unlock", h.Unlock, jwt, limiter)
	v1.POST("/auth/pin", h.ChangePin, jwt, middleware.RequireUnlocked())
	v1.GET("/me", h.Me, jwt)

	h.registerKiosk(v1, jwt) // Task 6
	h.registerSystem(v1)     // Task 6
	h.registerTill(v1)

	e.Any("/api/*", func(echo.Context) error { return apperr.NotFound("endpoint") })
}
