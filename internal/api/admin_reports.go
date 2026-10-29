package api

import (
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
)

// registerAdminReports mounts /admin/reports/*.
// Each report is one GET answering JSON, or a sheet when ?format= is set
// (writeSheet over every row in range, never one page).
func (h *handlers) registerAdminReports(g *echo.Group) {
	h.registerReportsSales(g)
	h.registerReportsProducts(g)
	h.registerReportsPeople(g)
	h.registerReportsMoney(g)
	h.registerReportsStock(g)
	h.registerReportsOps(g)
	h.registerAdminActivity(g)
	h.registerAdminDashboard(g)
}

// reportRange is a report's period: a preset or from / to (to inclusive of its
// whole day, reports.ResolveRange). Nothing chosen = this month.
// fs / ts are the resolved dates for the page's caption and the sheet's name.
func reportRange(c echo.Context) (from, to time.Time, fs, ts string, err error) {
	preset := c.QueryParam("preset")
	if preset == "" && c.QueryParam("from") == "" && c.QueryParam("to") == "" {
		preset = "this-month"
	}
	from, to, fs, ts, err = reports.ResolveRange(preset, c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		err = apperr.Validation(err.Error())
	}
	return
}

// prevPeriod is the equal-length period just before [from, to): a 7-day range
// compares with the 7 days before it (the ▲▼ deltas).
func prevPeriod(from, to time.Time) (time.Time, time.Time) {
	return from.Add(-to.Sub(from)), from
}

// wantsSheet: the report was asked for as a download (csv / xlsx / ods).
func wantsSheet(c echo.Context) bool { return c.QueryParam("format") != "" }
