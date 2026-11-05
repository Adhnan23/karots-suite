package pocketcashier

import (
	"strconv"
	"time"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

const recentPage = 200

// recent hands a phone the next page of the last 30 days' sales. The cursor is
// the phone's: it asks from where its last reply left off.
func (h handlers) recent(c echo.Context) error {
	var after int64
	var since time.Time
	var err error
	if a := c.QueryParam("after"); a != "" {
		if after, err = strconv.ParseInt(a, 10, 64); err != nil {
			return apperr.BadRequest("invalid cursor")
		}
	}
	if s := c.QueryParam("since"); s != "" {
		if since, err = time.Parse(time.RFC3339Nano, s); err != nil {
			return apperr.BadRequest("invalid cursor")
		}
	}
	page, err := h.p.core.RecentSales(c.Request().Context(), after, since, c.QueryParam("at"), recentPage)
	if err != nil {
		return err
	}
	return sendZipped(c, page)
}
