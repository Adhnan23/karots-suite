package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

func guardErr(t *testing.T, mw echo.MiddlewareFunc, set func(echo.Context)) *apperr.AppError {
	t.Helper()
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil), httptest.NewRecorder())
	set(c)
	err := mw(func(echo.Context) error { return nil })(c)
	if err == nil {
		return nil
	}
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("not an AppError: %v", err)
	}
	return ae
}

func TestRequireUnlockedCode(t *testing.T) {
	ae := guardErr(t, RequireUnlocked(), func(c echo.Context) { c.Set(ctxLocked, true) })
	if ae == nil || ae.Status != 403 || ae.Code != "SESSION_LOCKED" {
		t.Fatalf("got %+v", ae)
	}
	if guardErr(t, RequireUnlocked(), func(echo.Context) {}) != nil {
		t.Fatal("unlocked session was blocked")
	}
}

func TestRequirePinChosenCode(t *testing.T) {
	ae := guardErr(t, RequirePinChosen(), func(c echo.Context) { c.Set(ctxMustChangePin, true) })
	if ae == nil || ae.Status != 403 || ae.Code != "PIN_CHANGE_REQUIRED" {
		t.Fatalf("got %+v", ae)
	}
}
