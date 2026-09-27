package middleware

import (
	"errors"
	"net/http"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// ErrorHandler is installed as Echo's central HTTPErrorHandler. karots-suite has
// no server-rendered pages, so every error — API or not — is the JSON shape the
// SPA understands: {"error":{"code","message"}}. The SPA decides where to
// navigate (login/lock/pin) from the code; the server never redirects.
func ErrorHandler(isProd bool) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status, code, msg := classify(err, isProd)
		_ = c.JSON(status, map[string]any{
			"error": map[string]string{"code": code, "message": msg},
		})
	}
}

func classify(err error, isProd bool) (status int, code, msg string) {
	if ae, ok := apperr.As(err); ok {
		return ae.Status, ae.Code, ae.Message
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		m := http.StatusText(he.Code)
		if s, ok := he.Message.(string); ok && s != "" {
			m = s
		}
		return he.Code, "HTTP_ERROR", m
	}
	if isProd {
		return http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong"
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR", err.Error()
}
