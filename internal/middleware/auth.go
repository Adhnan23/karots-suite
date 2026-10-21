package middleware

import (
	"context"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// UserValidator reports whether the user behind a verified token may still act —
// i.e. the account still exists and is active — and returns their per-user
// flags. It is injected so this package keeps no dependency on the auth feature.
type UserValidator func(ctx context.Context, userID int64) (UserFlags, bool)

// userValidator is set once at startup (SetUserValidator) and consulted by every
// JWTAuth instance. A package-level hook keeps JWTAuth's signature unchanged, so
// the many feature API routes that build their own JWTAuth all get the check for
// free. When nil, tokens are trusted on signature + expiry alone.
var userValidator UserValidator

// SetUserValidator installs the account-still-active check used by all JWTAuth
// middleware. Call it once during startup, before serving requests.
func SetUserValidator(v UserValidator) { userValidator = v }

const (
	ctxUserID        = "user_id"
	ctxRole          = "role"
	ctxName          = "user_name"
	ctxMustChangePin = "must_change_pin"
	ctxLocked        = "locked"

	// CookieName is the httpOnly cookie that carries the access token for the
	// server-rendered UI.
	CookieName = "pos_token"
)

// Claims is the JWT payload. Kept here (not in the auth feature) so middleware
// has no dependency on feature packages.
type Claims struct {
	UserID        int64  `json:"uid"`
	Role          string `json:"role"`
	Name          string `json:"name"`
	MustChangePin bool   `json:"mcp,omitempty"`
	Locked        bool   `json:"lck,omitempty"`
	jwt.RegisteredClaims
}

// JWTAuth authenticates a request from either a Bearer header (API clients) or
// the httpOnly cookie (UI). On failure it returns an apperr the error handler
// converts to a 401/redirect; this keeps the middleware free of rendering
// concerns.
func JWTAuth(secret string) echo.MiddlewareFunc {
	key := []byte(secret)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenStr := extractToken(c)
			if tokenStr == "" {
				return apperr.Unauthorized("")
			}
			claims := &Claims{}
			_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, apperr.Unauthorized("invalid signing method")
				}
				return key, nil
			})
			if err != nil {
				return apperr.Unauthorized("invalid or expired session")
			}
			// A valid signature isn't enough: the account may have been deleted or
			// deactivated since the token was issued. Reject it so a fired/disabled
			// user is forced out instead of riding their cookie until it expires.
			var flags UserFlags
			if userValidator != nil {
				f, ok := userValidator(c.Request().Context(), claims.UserID)
				if !ok {
					return apperr.Unauthorized("your account is no longer active — please sign in again")
				}
				if (f.Role != "" && f.Role != claims.Role) || (f.Name != "" && f.Name != claims.Name) {
					return apperr.Unauthorized("your account changed — please sign in again")
				}
				flags = f
			}
			c.Set(ctxUserID, claims.UserID)
			c.Set(ctxRole, claims.Role)
			c.Set(ctxName, claims.Name)
			c.Set(ctxMustChangePin, claims.MustChangePin)
			c.Set(ctxLocked, claims.Locked)
			c.Set(ctxCanSuppliers, flags.CanHandleSuppliers)
			c.Set(ctxCanManageCredit, flags.CanManageCredit)
			c.Set(ctxCanSeeCost, flags.CanSeeCost)
			c.Set(ctxCanSeeLowStock, flags.CanSeeLowStock)
			// Templates read the flags from the request context (see flags.go),
			// which response.render hands to templ.
			c.SetRequest(c.Request().WithContext(
				context.WithValue(c.Request().Context(), ctxFlagsKey, flags)))
			return next(c)
		}
	}
}

func extractToken(c echo.Context) string {
	if h := c.Request().Header.Get(echo.HeaderAuthorization); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if cookie, err := c.Cookie(CookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// ParseClaims validates a token's signature + expiry and returns its claims. It
// does NOT run the user-active check, so it is suitable for handlers that are
// intentionally ungated (e.g. logout) but still want to know who the caller is.
// Returns false when the token is missing, malformed, expired, or wrongly signed.
func ParseClaims(c echo.Context, secret string) (*Claims, bool) {
	tokenStr := extractToken(c)
	if tokenStr == "" {
		return nil, false
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperr.Unauthorized("invalid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, false
	}
	return claims, true
}

// RequireRole authorizes the request against an allowlist of roles. Must run
// after JWTAuth.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get(ctxRole).(string)
			if _, ok := allowed[role]; !ok {
				return apperr.Forbidden("")
			}
			return next(c)
		}
	}
}

// CurrentUserID returns the authenticated user's id, or 0 if unauthenticated.
func CurrentUserID(c echo.Context) int64 {
	id, _ := c.Get(ctxUserID).(int64)
	return id
}

// CurrentRole returns the authenticated user's role, or "".
func CurrentRole(c echo.Context) string {
	r, _ := c.Get(ctxRole).(string)
	return r
}

// CurrentUserName returns the authenticated user's display name, or "".
func CurrentUserName(c echo.Context) string {
	n, _ := c.Get(ctxName).(string)
	return n
}

// MustChangePin reports whether the authenticated user is required to change
// their PIN before using the rest of the app.
func MustChangePin(c echo.Context) bool {
	b, _ := c.Get(ctxMustChangePin).(bool)
	return b
}

// RequirePinChosen blocks a user still carrying a server-assigned PIN. The SPA
// routes PIN_CHANGE_REQUIRED to its change-PIN screen. Must run after JWTAuth.
func RequirePinChosen() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if MustChangePin(c) {
				return &apperr.AppError{Status: http.StatusForbidden, Code: "PIN_CHANGE_REQUIRED", Message: "Choose a new PIN to continue"}
			}
			return next(c)
		}
	}
}

// IsLocked reports whether the current session is locked (screen lock, NOT a
// logout — the identity and any open till/float are intact).
func IsLocked(c echo.Context) bool {
	b, _ := c.Get(ctxLocked).(bool)
	return b
}

// RequireUnlocked blocks a screen-locked session (identity + open till intact).
// The SPA routes SESSION_LOCKED to its lock screen. Must run after JWTAuth.
func RequireUnlocked() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if IsLocked(c) {
				return &apperr.AppError{Status: http.StatusForbidden, Code: "SESSION_LOCKED", Message: "Session locked"}
			}
			return next(c)
		}
	}
}
