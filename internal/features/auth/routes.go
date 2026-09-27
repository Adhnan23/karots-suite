package auth

import (
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// NewLoginLimiter returns a rate limiter sized to blunt PIN brute-forcing.
// PINs are low entropy, so login endpoints (UI and API) share this policy.
func NewLoginLimiter() echo.MiddlewareFunc {
	return echomw.RateLimiterWithConfig(echomw.RateLimiterConfig{
		Store: echomw.NewRateLimiterMemoryStoreWithConfig(echomw.RateLimiterMemoryStoreConfig{
			Rate:      0.5,
			Burst:     10,
			ExpiresIn: 3 * time.Minute,
		}),
	})
}
