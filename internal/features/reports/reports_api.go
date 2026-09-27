// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/config"
	"karots-suite/internal/middleware"
	"karots-suite/internal/response"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type APIHandler struct{ svc *Service }

func NewAPIHandler(svc *Service) *APIHandler { return &APIHandler{svc: svc} }

func (h *APIHandler) Finance(c echo.Context) error {
	from, to, err := ParseRange(c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return err
	}
	pl, err := h.svc.Compute(c.Request().Context(), from, to)
	if err != nil {
		return err
	}
	return response.OK(c, pl)
}

// ParseRange parses optional from/to (YYYY-MM-DD), defaulting to the current
// calendar month. `to` is made exclusive (end of day).
func ParseRange(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 1, 0)
	if fromStr != "" {
		t, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return from, to, apperr.BadRequest("from must be YYYY-MM-DD")
		}
		from = t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return from, to, apperr.BadRequest("to must be YYYY-MM-DD")
		}
		to = t.AddDate(0, 0, 1)
	}
	return from, to, nil
}

// ResolveRange turns an optional quick-pick preset (today, this-week, this-month,
// last-week, last-month, this-year) into a [from, to) range plus the inclusive
// YYYY-MM-DD display strings for the form. A non-empty preset wins; otherwise it
// falls back to ParseRange(fromStr, toStr). Weeks run Monday–Sunday. The returned
// `to` is exclusive (start of the day after the last day in range).
func ResolveRange(preset, fromStr, toStr string) (from, to time.Time, fromOut, toOut string, err error) {
	if preset == "" {
		from, to, err = ParseRange(fromStr, toStr)
		if err != nil {
			return
		}
		fromOut = fromStr
		if fromOut == "" {
			fromOut = from.Format("2006-01-02")
		}
		toOut = toStr
		if toOut == "" {
			toOut = to.AddDate(0, 0, -1).Format("2006-01-02")
		}
		return
	}

	now := time.Now()
	loc := now.Location()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	switch preset {
	case "today":
		from, to = day, day.AddDate(0, 0, 1)
	case "this-week":
		from = weekStart(day)
		to = from.AddDate(0, 0, 7)
	case "last-week":
		to = weekStart(day)
		from = to.AddDate(0, 0, -7)
	case "this-month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		to = from.AddDate(0, 1, 0)
	case "last-month":
		to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		from = to.AddDate(0, -1, 0)
	case "this-year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		to = from.AddDate(1, 0, 0)
	default:
		err = apperr.BadRequest("unknown preset")
		return
	}
	fromOut = from.Format("2006-01-02")
	toOut = to.AddDate(0, 0, -1).Format("2006-01-02")
	return
}

// weekStart returns the Monday 00:00 of the week containing d.
func weekStart(d time.Time) time.Time {
	off := (int(d.Weekday()) + 6) % 7 // Mon=0 … Sun=6
	return d.AddDate(0, 0, -off)
}

func RegisterAPI(e *echo.Echo, db *sqlx.DB, cfg *config.Config) {
	api := NewAPIHandler(NewService(db))
	g := e.Group("/api/reports", middleware.JWTAuth(cfg.JWTSecret), middleware.RequireRole("admin", "manager"))
	g.GET("/finance", api.Finance)
}
