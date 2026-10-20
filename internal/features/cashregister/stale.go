package cashregister

import (
	"time"

	"karots-suite/internal/datetime"
)

// StaleAfter is how long a session from an earlier day must have been open before it
// counts as left open, so a shop trading past midnight isn't stopped at 00:00.
// ponytail: fixed; a setting if a night shop ever needs a longer one.
const StaleAfter = 6 * time.Hour

// Stale reports whether a session opened at openedAt was left open: opened before
// today (shop time) and at least StaleAfter ago. Its cashier must close it before
// selling on; an admin or manager may close it for them.
func Stale(openedAt, now time.Time) bool {
	n := now.In(datetime.Location)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, datetime.Location)
	return openedAt.Before(today) && now.Sub(openedAt) >= StaleAfter
}

// ClosedLate reports whether a session was closed on a later shop day than it opened.
func ClosedLate(openedAt time.Time, closedAt *time.Time) bool {
	if closedAt == nil {
		return false
	}
	a, b := openedAt.In(datetime.Location), closedAt.In(datetime.Location)
	return a.Year() != b.Year() || a.YearDay() != b.YearDay()
}
