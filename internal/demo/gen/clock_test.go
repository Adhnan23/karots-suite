package gen

import (
	"testing"
	"time"
)

// A past whose last trade was two days ago moves by one day in any time zone —
// Sri Lanka is UTC+5:30, so local midnight is the previous day in UTC.
func TestDaysToMoveCountsLocalDays(t *testing.T) {
	colombo := time.FixedZone("LKT", 5*3600+1800)
	for _, now := range []time.Time{
		time.Date(2026, 10, 27, 0, 5, 0, 0, colombo),
		time.Date(2026, 10, 27, 9, 0, 0, 0, colombo),
		time.Date(2026, 10, 27, 23, 50, 0, 0, colombo),
	} {
		last := time.Date(2026, 10, 25, 20, 30, 0, 0, colombo).UTC() // stored in UTC
		if got := daysToMove(now, last); got != 1 {
			t.Errorf("now %v: move %d days, want 1", now, got)
		}
	}
}
