// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
)

// ParseRange parses optional from/to (YYYY-MM-DD), defaulting to the current
// calendar month. `to` is made exclusive (end of day). Days are the shop's days
// (datetime.Location), not the server process's zone or UTC.
func ParseRange(fromStr, toStr string) (time.Time, time.Time, error) {
	now := time.Now().In(datetime.Location)
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, datetime.Location)
	to := from.AddDate(0, 1, 0)
	if fromStr != "" {
		t, err := time.ParseInLocation("2006-01-02", fromStr, datetime.Location)
		if err != nil {
			return from, to, apperr.BadRequest("from must be YYYY-MM-DD")
		}
		from = t
	}
	if toStr != "" {
		t, err := time.ParseInLocation("2006-01-02", toStr, datetime.Location)
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

	loc := datetime.Location
	now := time.Now().In(loc)
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
