package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/presence"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// deviceHeader carries the browser's random device id (localStorage ks-device) on every call.
const deviceHeader = "X-Ks-Device"

func device(c echo.Context) string {
	d := c.Request().Header.Get(deviceHeader)
	if len(d) > 64 {
		return ""
	}
	return d
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// PresenceBeat records a heartbeat. jwt-only, so a locked screen keeps reporting;
// the hidden system user never shows.
func (h *handlers) PresenceBeat(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if !ok || claims.UserID == 0 {
		return apperr.Unauthorized("")
	}
	var req PresenceBeat
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if !middleware.IsSystemUserCtx(c.Request().Context()) {
		h.presence.Beat(presence.Beat{
			Device: device(c), UserID: claims.UserID, Name: claims.Name, Role: claims.Role, Locked: claims.Locked,
			Path: clip(req.Path, 200), IdleSec: req.IdleSec, CartItems: req.CartItems, CartTotal: clip(req.CartTotal, 20), Note: clip(req.Note, 80),
			IP: c.RealIP(), UA: clip(c.Request().UserAgent(), 300),
		}, time.Now())
	}
	return c.NoContent(http.StatusNoContent)
}

// AdminPresence is "Who's on now" (admin only): the live devices plus each user's
// open till and sales so far today.
func (h *handlers) AdminPresence(c echo.Context) error {
	ctx := c.Request().Context()
	tills := map[int64]time.Time{}
	var open []struct {
		UserID   int64     `db:"user_id"`
		OpenedAt time.Time `db:"opened_at"`
	}
	if err := h.d.DB.SelectContext(ctx, &open, `SELECT user_id, opened_at FROM cash_register WHERE closed_at IS NULL`); err != nil {
		return err
	}
	for _, o := range open {
		tills[o.UserID] = o.OpenedAt
	}
	sales := map[int64]int{}
	var counts []struct {
		UserID int64 `db:"cashier_id"`
		N      int   `db:"n"`
	}
	if err := h.d.DB.SelectContext(ctx, &counts, `SELECT cashier_id, COUNT(*) AS n FROM sales
		WHERE created_at >= $1 AND status <> 'void' GROUP BY cashier_id`, shopDay(time.Now())); err != nil {
		return err
	}
	for _, n := range counts {
		sales[n.UserID] = n.N
	}
	out := []PresenceRow{}
	for _, r := range h.presence.List(time.Now()) {
		row := PresenceRow{Device: r.Device, UserID: r.UserID, Name: r.Name, Role: r.Role, State: r.State, Path: r.Path,
			IdleSec: r.IdleSec, CartItems: r.CartItems, CartTotal: r.CartTotal, Note: r.Note, IP: r.IP, Label: r.Label,
			LastSeen: r.LastSeen.UTC().Format(time.RFC3339), SalesToday: sales[r.UserID]}
		if t, ok := tills[r.UserID]; ok {
			s := t.UTC().Format(time.RFC3339)
			row.TillOpened = &s
		}
		out = append(out, row)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) registerPresence(v1 *echo.Group, jwt echo.MiddlewareFunc) {
	v1.POST("/presence", h.PresenceBeat, jwt)
	v1.GET("/admin/presence", h.AdminPresence, append(h.protected, middleware.RequireRole(auth.RoleAdmin))...)
}
