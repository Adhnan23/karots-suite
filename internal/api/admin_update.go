package api

import (
	"net/http"

	"karots-suite/internal/apperr"
	"karots-suite/internal/update"

	"github.com/labstack/echo/v4"
)

// Updates card on Setup → Data: status, check now, install, dismiss the
// "went back to the old version" notice. Admin only (the /data group).
func (h *handlers) registerAdminUpdate(d *echo.Group) {
	d.GET("/update", h.AdminUpdateStatus)
	d.POST("/update/check", h.AdminUpdateCheck)
	d.POST("/update/apply", h.AdminUpdateApply)
	d.POST("/update/dismiss", h.AdminUpdateDismiss)
}

func (h *handlers) updateStatus() UpdateStatusDTO {
	u := update.Active()
	if u == nil {
		return UpdateStatusDTO{Version: h.d.Cfg.Version, State: string(update.StateUnchecked)}
	}
	s := u.Status()
	dto := UpdateStatusDTO{Version: s.Current, Kind: s.Kind, Enabled: true, State: string(s.State), Latest: s.Latest,
		Notes: s.Notes, Message: s.Message, Progress: s.Progress, RolledBackFrom: s.RolledBackFrom}
	if !s.CheckedAt.IsZero() {
		dto.CheckedAt = &s.CheckedAt
	}
	return dto
}

func (h *handlers) AdminUpdateStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, h.updateStatus())
}

func (h *handlers) AdminUpdateCheck(c echo.Context) error {
	if u := update.Active(); u != nil {
		_ = u.Check(c.Request().Context()) // the outcome is in the status
	}
	return c.JSON(http.StatusOK, h.updateStatus())
}

func (h *handlers) AdminUpdateApply(c echo.Context) error {
	u := update.Active()
	if u == nil {
		return apperr.Validation("this copy of Karots doesn't update itself")
	}
	if err := u.Apply(c.Request().Context()); err != nil {
		return apperr.Validation(err.Error())
	}
	return c.JSON(http.StatusAccepted, h.updateStatus())
}

func (h *handlers) AdminUpdateDismiss(c echo.Context) error {
	if u := update.Active(); u != nil {
		u.Dismiss()
	}
	return c.JSON(http.StatusOK, h.updateStatus())
}
