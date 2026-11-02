package api

import (
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/presence"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// One person, one device (on only when a plugin asks for it): signing in on a
// device makes it the person's one device, and their other devices answer
// SESSION_LOCKED until they sign in there again.

// movedTo is the label of the device the signed-in person moved to, or "" when
// this request comes from their device (or the rule is off).
func movedTo(c echo.Context) string {
	ctx := c.Request().Context()
	if !plugin.OneDevice() || middleware.IsSystemUserCtx(ctx) {
		return ""
	}
	dev, label := middleware.ActiveDeviceCtx(ctx)
	// A call naming no device (a script, a test's API request) can't be told
	// apart; only one naming a different device has moved away.
	if here := device(c); dev == "" || here == "" || dev == here {
		return ""
	}
	if label == "" {
		label = "another device"
	}
	return label
}

// onDevice refuses a request from a device the person has left. Parking the
// cart (POST /till/held) still works, so a moved till can hold what was on it, and
// the ticket poll isn't what sends it to the lock screen before it has.
func (h *handlers) onDevice(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		to := movedTo(c)
		if to == "" || (c.Request().Method == http.MethodPost && c.Path() == "/api/v1/till/held") || c.Path() == "/api/v1/till/tickets" {
			return next(c)
		}
		return &apperr.AppError{Status: http.StatusForbidden, Code: "SESSION_LOCKED", Message: middleware.CurrentUserName(c) + " is now on " + to}
	}
}

// takeDevice makes this device the user's one device, unless a plugin guard
// holds them where they are (409 DEVICE_HELD with its reason).
func (h *handlers) takeDevice(c echo.Context, userID int64, label string) error {
	if !plugin.OneDevice() {
		return nil
	}
	ctx := c.Request().Context()
	active, system, err := h.d.Auth.DeviceOf(ctx, userID)
	if err != nil || system {
		return err
	}
	dev := device(c)
	if active != "" && active != dev {
		for _, g := range plugin.TakeoverGuards() {
			if block, reason := g(ctx, userID, active); block {
				return apperr.ConflictCode("DEVICE_HELD", reason)
			}
		}
	}
	if label = strings.TrimSpace(label); label == "" {
		label = presence.Label(c.Request().UserAgent())
	}
	return h.d.Auth.ClaimDevice(ctx, userID, dev, clip(label, 60))
}
