// Package app holds the parts of server startup shared by every entry point:
// migrating the database and assembling the fully-wired Echo server. cmd/server
// (the real POS) and cmd/demo (the laptop showcase) both call these, so the
// service-wiring — which core seams are injected into which service — lives in
// exactly one place and the demo can never drift from the product.
package app

import (
	"log"
	"net/http"
	"time"

	"karots-suite/internal/api"
	"karots-suite/internal/config"
	"karots-suite/internal/db"
	appmw "karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/spa"
	"karots-suite/internal/validator"
	"karots-suite/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// Migrate brings a connected database fully up to date: core migrations first,
// then each enabled plugin's own migration set (each owns its own goose version
// table, so this is the idempotent late-enable path). Runs on every boot.
func Migrate(sqlxDB *sqlx.DB) error {
	if err := db.RunMigrations(sqlxDB.DB, migrations.FS); err != nil {
		return err
	}
	for _, p := range plugin.All() {
		pfs, suffix := p.Migrations()
		if pfs == nil || suffix == "" {
			continue
		}
		if err := db.RunMigrationsTable(sqlxDB.DB, pfs, "goose_db_version_"+suffix); err != nil {
			return err
		}
		log.Printf("plugin %q: migrations applied (goose_db_version_%s)", p.Name(), suffix)
	}
	return nil
}

// BuildServer assembles the complete HTTP server: static assets, health probe,
// every feature's JSON API with its core seams wired, and the HTMX/Templ UI.
// systemPIN is the login check for the hidden System account — the real server
// passes its hourly-rotating validator, the demo passes a fixed one.
//
// pre are extra middlewares registered before any route, so they see every
// request. cmd/server passes none; cmd/demo passes its DEMO-chrome injector. It
// is the only demo-specific seam into the otherwise-identical server.
func BuildServer(sqlxDB *sqlx.DB, cfg *config.Config, systemPIN func(pin string, now time.Time) bool, pre ...echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	// No proxy in front of a shop binary: trust the socket, never X-Forwarded-For,
	// or the login rate limiter (PIN brute-force guard) is bypassed by a spoofed header.
	e.IPExtractor = echo.ExtractIPDirect()
	e.Validator = validator.New()
	e.HTTPErrorHandler = appmw.ErrorHandler(cfg.IsProd())

	e.Use(echomw.Recover())
	e.Use(versionHeader(cfg.Version))
	e.Use(echomw.Logger())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{AllowOrigins: cfg.CORSOrigins}))
	for _, mw := range pre {
		e.Use(mw)
	}

	// Liveness probe for deployments: 200 only when the DB answers a ping.
	e.GET("/health", func(c echo.Context) error {
		if err := sqlxDB.PingContext(c.Request().Context()); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "down", "db": "unreachable"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Account-still-active check consulted by every JWTAuth instance; also
	// resolves the per-user flags (CanExitKiosk, IsSystem, …) that /me and the
	// route guards read from the request context.
	appmw.SetUserValidator(appmw.DBUserValidator(sqlxDB))

	svc := NewServices(sqlxDB, cfg, systemPIN)
	mountPlugins := startPlugins(e, sqlxDB, cfg, svc)
	api.Register(e, api.Deps{DB: sqlxDB, Cfg: cfg, Auth: svc.Auth, Settings: svc.Settings, CashRegister: svc.CashRegister, Cashflow: svc.Cashflow, Sales: svc.Sales, KickDrawer: svc.KickDrawer, MountPlugins: mountPlugins})
	e.GET("/*", spa.Handler(spa.Dist()))

	return e
}

// versionHeader stamps every response with the binary's version, so an open
// SPA notices an update and reloads itself (web/src/lib/version.ts).
func versionHeader(version string) echo.MiddlewareFunc {
	if version == "" {
		version = "dev"
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Karots-Version", version)
			return next(c)
		}
	}
}
