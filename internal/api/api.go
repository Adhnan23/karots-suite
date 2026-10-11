// Package api is karots-suite's JSON HTTP layer (/api/v1). Handlers are thin:
// bind → call a domain service → return a DTO from dto.go. One file per area;
// every protected area group uses h.protected.
package api

import (
	"context"
	"slices"

	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"
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
	Cashflow     *cashflow.Service
	Sales        *sales.Service
	KickDrawer   func(context.Context)
	// MountPlugins mounts the compiled-in plugins' routes onto /api/v1/x groups
	// carrying core's guards (public, signed-in, admin + manager). nil = none.
	MountPlugins func(public, cashier, admin *echo.Group)
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
	h.registerAdmin(v1)
	h.registerLegacySync(e, limiter, jwt)
	if d.MountPlugins != nil {
		admin := slices.Concat(h.protected, []echo.MiddlewareFunc{middleware.RequireRole(auth.RoleAdmin, auth.RoleManager)})
		d.MountPlugins(v1.Group("/x"), v1.Group("/x", h.protected...), v1.Group("/x", admin...))
	}

	e.Any("/api/*", func(echo.Context) error { return apperr.NotFound("endpoint") })
}
