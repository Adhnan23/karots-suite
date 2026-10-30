package pocketcashier

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

type handlers struct{ p *Plugin }

type PhoneRow struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	UserName string `json:"userName"`
	CopyAt   string `json:"copyAt"`
	LastSeen string `json:"lastSeen"`
	PairCode string `json:"pairCode"`
}

type ApproveRequest struct {
	Code string `json:"code"`
}

type RenameRequest struct {
	Name string `json:"name"`
}

type Addresses struct {
	Setup []string `json:"setup"`
	App   []string `json:"app"`
}

func iso(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func (h handlers) audit(c echo.Context, action string, id int64, detail string) {
	h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "phone", strconv.FormatInt(id, 10), detail)
}

func (h handlers) phones(c echo.Context) error {
	var rows []struct {
		ID       int64      `db:"id"`
		Name     string     `db:"name"`
		Status   string     `db:"status"`
		UserName *string    `db:"user_name"`
		CopyAt   *time.Time `db:"copy_at"`
		LastSeen *time.Time `db:"last_seen"`
		PairCode *string    `db:"pair_code"`
	}
	if err := h.p.core.DB.SelectContext(c.Request().Context(), &rows, `
		SELECT p.id, p.name, p.status, u.name AS user_name, p.copy_at, p.last_seen, p.pair_code
		FROM pocket_phones p LEFT JOIN users u ON u.id = p.user_id
		ORDER BY p.status = 'pending' DESC, p.status = 'blocked', p.name, p.id`); err != nil {
		return apperr.Internal("failed to list phones", err)
	}
	out := make([]PhoneRow, 0, len(rows))
	for _, r := range rows {
		row := PhoneRow{ID: r.ID, Name: r.Name, Status: r.Status, CopyAt: iso(r.CopyAt), LastSeen: iso(r.LastSeen)}
		if r.UserName != nil {
			row.UserName = *r.UserName
		}
		if r.PairCode != nil && r.Status == "pending" {
			row.PairCode = *r.PairCode
		}
		out = append(out, row)
	}
	return c.JSON(http.StatusOK, out)
}

// approve turns a waiting phone on: by the code it shows (typed or scanned), or by id.
func (h handlers) approve(c echo.Context) error {
	ctx := c.Request().Context()
	q, arg := `WHERE status = 'pending' AND pair_code = $2`, any(nil)
	if id, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil {
		q, arg = `WHERE status = 'pending' AND id = $2`, id
	} else {
		var req ApproveRequest
		if err := c.Bind(&req); err != nil {
			return apperr.BadRequest("invalid request body")
		}
		arg = strings.TrimSpace(req.Code)
	}
	var id int64
	var name string
	err := h.p.core.DB.QueryRowxContext(ctx, `UPDATE pocket_phones SET status = 'active', pair_code = NULL, approved_at = now(), approved_by = $1 `+q+
		` RETURNING id, name`, middleware.CurrentUserID(c), arg).Scan(&id, &name)
	if err != nil {
		return apperr.Validation("No phone is waiting with that code")
	}
	h.audit(c, audit.ActionCreate, id, "approved phone "+name)
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) phoneID(c echo.Context) (int64, string, *int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, "", nil, apperr.NotFound("phone")
	}
	var row struct {
		Name   string `db:"name"`
		UserID *int64 `db:"user_id"`
	}
	if err := h.p.core.DB.GetContext(c.Request().Context(), &row, `SELECT name, user_id FROM pocket_phones WHERE id = $1`, id); err != nil {
		return 0, "", nil, apperr.NotFound("phone")
	}
	return id, row.Name, row.UserID, nil
}

// block stops the phone's key at once (its person is released too).
func (h handlers) block(c echo.Context) error {
	id, name, uid, err := h.phoneID(c)
	if err != nil {
		return err
	}
	if err := h.releaseUser(c, id, uid); err != nil {
		return err
	}
	if _, err := h.p.core.DB.ExecContext(c.Request().Context(), `UPDATE pocket_phones SET status = 'blocked', pair_code = NULL WHERE id = $1`, id); err != nil {
		return apperr.Internal("failed to block the phone", err)
	}
	h.audit(c, audit.ActionUpdate, id, "blocked phone "+name)
	return c.NoContent(http.StatusNoContent)
}

// release frees the person on a lost or dead phone, so they can sign in elsewhere.
func (h handlers) release(c echo.Context) error {
	id, name, uid, err := h.phoneID(c)
	if err != nil {
		return err
	}
	if err := h.releaseUser(c, id, uid); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, id, "released the person on phone "+name)
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) releaseUser(c echo.Context, id int64, uid *int64) error {
	ctx := c.Request().Context()
	if uid != nil {
		// Only when the person is still held by this phone.
		var dev string
		_ = h.p.core.DB.GetContext(ctx, &dev, `SELECT COALESCE(u.active_device, '') FROM users u JOIN pocket_phones p ON p.device = u.active_device WHERE u.id = $1 AND p.id = $2`, *uid, id)
		if dev != "" {
			if err := h.p.core.Auth.ReleaseDevice(ctx, *uid); err != nil {
				return err
			}
		}
	}
	if _, err := h.p.core.DB.ExecContext(ctx, `UPDATE pocket_phones SET user_id = NULL WHERE id = $1`, id); err != nil {
		return apperr.Internal("failed to release the phone", err)
	}
	return nil
}

func (h handlers) rename(c echo.Context) error {
	id, _, _, err := h.phoneID(c)
	if err != nil {
		return err
	}
	var req RenameRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 40 {
		return apperr.Validation("give the phone a name (up to 40 letters)")
	}
	if _, err := h.p.core.DB.ExecContext(c.Request().Context(), `UPDATE pocket_phones SET name = $2 WHERE id = $1`, id, name); err != nil {
		return apperr.Internal("failed to rename the phone", err)
	}
	h.audit(c, audit.ActionUpdate, id, "renamed phone to "+name)
	return c.NoContent(http.StatusNoContent)
}

// remove forgets a phone (pairing it again starts over).
func (h handlers) remove(c echo.Context) error {
	id, name, uid, err := h.phoneID(c)
	if err != nil {
		return err
	}
	if err := h.releaseUser(c, id, uid); err != nil {
		return err
	}
	if _, err := h.p.core.DB.ExecContext(c.Request().Context(), `DELETE FROM pocket_phones WHERE id = $1`, id); err != nil {
		return apperr.Internal("failed to remove the phone", err)
	}
	h.audit(c, audit.ActionDelete, id, "removed phone "+name)
	return c.NoContent(http.StatusNoContent)
}

// addresses: where a phone opens the setup page (plain HTTP, the normal port)
// and the app (HTTPS, the phones' port), one per LAN IP.
func (h handlers) addresses(c echo.Context) error {
	out := Addresses{Setup: []string{}, App: []string{}}
	for _, ip := range lanIPs() {
		if ip.IsLoopback() {
			continue
		}
		out.Setup = append(out.Setup, fmt.Sprintf("http://%s:%d/x/pocketcashier/setup", ip, h.p.core.Cfg.ServerPort))
		out.App = append(out.App, fmt.Sprintf("https://%s:%d/x/pocketcashier/", ip, Port))
	}
	return c.JSON(http.StatusOK, out)
}

var adminOnly = middleware.RequireRole(auth.RoleAdmin)
