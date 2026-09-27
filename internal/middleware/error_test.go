package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

func runErr(t *testing.T, err error, isProd bool) (int, map[string]map[string]string) {
	t.Helper()
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/admin/x", nil), rec)
	ErrorHandler(isProd)(err, c)
	var body map[string]map[string]string
	if jerr := json.Unmarshal(rec.Body.Bytes(), &body); jerr != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	return rec.Code, body
}

func TestErrorHandlerAlwaysJSON(t *testing.T) {
	code, body := runErr(t, apperr.Unauthorized("nope"), true)
	if code != http.StatusUnauthorized || body["error"]["code"] == "" || body["error"]["message"] != "nope" {
		t.Fatalf("got %d %v", code, body)
	}
	if _, has := body["success"]; has {
		t.Fatal("legacy envelope key leaked")
	}
}

func TestErrorHandlerHidesInternalInProd(t *testing.T) {
	code, body := runErr(t, errors.New("pq: secret detail"), true)
	if code != 500 || body["error"]["message"] != "Something went wrong" {
		t.Fatalf("got %d %v", code, body)
	}
}

func TestErrorHandlerEchoHTTPError(t *testing.T) {
	code, body := runErr(t, echo.NewHTTPError(http.StatusNotFound, "Not Found"), false)
	if code != 404 || body["error"]["code"] != "HTTP_ERROR" {
		t.Fatalf("got %d %v", code, body)
	}
}
