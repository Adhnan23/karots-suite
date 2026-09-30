package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// registerAdminUsers mounts staff logins (karots-pos /admin/users) — admin only,
// on top of the admin group's admin-or-manager check.
func (h *handlers) registerAdminUsers(g *echo.Group) {
	u := g.Group("/users", middleware.RequireRole(auth.RoleAdmin))
	u.GET("", h.AdminUsers)
	u.POST("", h.AdminUserCreate)
	u.PUT("/:id", h.AdminUserUpdate)
	u.DELETE("/:id", h.AdminUserDeactivate)
	u.POST("/:id/activate", h.AdminUserReactivate)
}

func staffUser(u auth.User) StaffUser {
	return StaffUser{ID: u.ID, Name: u.Name, Phone: strOr(u.Phone), Role: u.Role, Active: u.IsActive, MustChangePin: u.MustChangePin,
		ReceiptPrinter: u.ReceiptPrinter, CanHandleSuppliers: u.CanHandleSuppliers, CanManageCredit: u.CanManageCredit, CanSeeCost: u.CanSeeCost,
		Since: u.CreatedAt.Format("2006-01-02")}
}

func (h *handlers) AdminUsers(c echo.Context) error {
	us, err := h.d.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]StaffUser, 0, len(us))
	for _, u := range us {
		out = append(out, staffUser(u))
	}
	return c.JSON(http.StatusOK, out)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return ""
}

func (h *handlers) AdminUserCreate(c echo.Context) error {
	var req StaffRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := auth.CreateUserInput{Name: strings.TrimSpace(req.Name), Phone: strings.TrimSpace(req.Phone), Role: req.Role, PIN: strings.TrimSpace(req.Pin),
		ReceiptPrinter: strings.TrimSpace(req.ReceiptPrinter), CanHandleSuppliers: onOff(req.CanHandleSuppliers), CanManageCredit: onOff(req.CanManageCredit), CanSeeCost: onOff(req.CanSeeCost)}
	if err := c.Validate(&in); err != nil {
		return err
	}
	u, err := h.d.Auth.CreateUser(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "user", strconv.FormatInt(u.ID, 10), "created user "+in.Name+" ("+in.Role+")")
	return c.JSON(http.StatusCreated, staffUser(*u))
}

// otherActiveAdmins counts active admins other than id — the shop must always
// keep one (an upgrade on karots-pos, which let the last admin lock everyone out).
func (h *handlers) otherActiveAdmins(c echo.Context, id int64) (int, error) {
	us, err := h.d.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range us {
		if u.ID != id && u.IsActive && u.Role == auth.RoleAdmin {
			n++
		}
	}
	return n, nil
}

func (h *handlers) AdminUserUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req StaffRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := auth.UpdateUserInput{Name: strings.TrimSpace(req.Name), Phone: strings.TrimSpace(req.Phone), Role: req.Role, PIN: strings.TrimSpace(req.Pin),
		ReceiptPrinter: strings.TrimSpace(req.ReceiptPrinter), CanHandleSuppliers: onOff(req.CanHandleSuppliers), CanManageCredit: onOff(req.CanManageCredit), CanSeeCost: onOff(req.CanSeeCost)}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if in.Role != auth.RoleAdmin {
		if n, err := h.otherActiveAdmins(c, id); err != nil {
			return err
		} else if n == 0 {
			if cur, err := h.d.Auth.GetUser(c.Request().Context(), id); err == nil && cur.IsActive && cur.Role == auth.RoleAdmin {
				return apperr.Validation("keep at least one admin — make someone else admin first")
			}
		}
	}
	if err := h.d.Auth.UpdateUser(c.Request().Context(), id, in); err != nil {
		return err
	}
	detail := "updated user " + in.Name + " (" + in.Role + ")"
	if in.PIN != "" {
		detail += ", PIN reset"
	}
	h.audit(c, audit.ActionUpdate, "user", strconv.FormatInt(id, 10), detail)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminUserDeactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if id == middleware.CurrentUserID(c) {
		return apperr.Validation("you can't deactivate yourself")
	}
	cur, err := h.d.Auth.GetUser(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if cur.Role == auth.RoleAdmin {
		if n, err := h.otherActiveAdmins(c, id); err != nil {
			return err
		} else if n == 0 {
			return apperr.Validation("keep at least one admin — make someone else admin first")
		}
	}
	if err := h.d.Auth.DeactivateUser(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "user", strconv.FormatInt(id, 10), "deactivated user")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminUserReactivate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.d.Auth.ReactivateUser(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "user", strconv.FormatInt(id, 10), "reactivated user")
	return c.NoContent(http.StatusNoContent)
}
