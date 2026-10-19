package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestVersionHeader(t *testing.T) {
	e := echo.New()
	e.Use(versionHeader("2026.10.19.1530"))
	e.GET("/ok", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/no", func(c echo.Context) error { return echo.ErrUnauthorized })
	for _, path := range []string{"/ok", "/no", "/missing"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Karots-Version"); got != "2026.10.19.1530" {
			t.Errorf("%s (%d): X-Karots-Version = %q", path, rec.Code, got)
		}
	}
}
