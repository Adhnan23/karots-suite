package api

import (
	"net/http"
	"strings"

	"karots-suite/internal/config"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// secureFlag decides the cookie's Secure attribute: "always"/"never" pin it,
// "auto" (the default) follows the actual connection so a plain-HTTP shop LAN
// still works while a shop genuinely behind TLS gets the protection.
func (h *handlers) secureFlag(c echo.Context) bool {
	switch h.d.Cfg.CookieSecure {
	case config.CookieSecureAlways:
		return true
	case config.CookieSecureNever:
		return false
	}
	r := c.Request()
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h *handlers) setCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     middleware.CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.d.Cfg.JWTAccessTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.secureFlag(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *handlers) clearCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     middleware.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureFlag(c),
		SameSite: http.SameSiteStrictMode,
	})
}
