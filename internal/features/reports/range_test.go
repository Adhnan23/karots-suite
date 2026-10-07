package reports

import (
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

// A day is the shop's day, whatever zone the server process runs in: a custom
// date and the "today" preset both start at the shop's midnight. (They used
// the process zone / UTC, so on a UTC host "to=7 Oct" ran until 05:30 on
// 8 Oct in Colombo and "from" missed the first 5½ hours.)
func TestRangesAreShopDays(t *testing.T) {
	old := time.Local
	time.Local = time.UTC // a UTC server
	defer func() { time.Local = old }()

	from, to, _, _, err := ResolveRange("", "2026-10-07", "2026-10-07")
	mustR(t, err)
	want := time.Date(2026, 10, 7, 0, 0, 0, 0, datetime.Location)
	if !from.Equal(want) || !to.Equal(want.AddDate(0, 0, 1)) {
		t.Fatalf("custom day = [%s, %s), want [%s, +1d)", from, to, want)
	}
	from, _, _, _, err = ResolveRange("today", "", "")
	mustR(t, err)
	if l := from.In(datetime.Location); l.Hour() != 0 || l.Minute() != 0 {
		t.Fatalf("today starts at %s shop time", l.Format("15:04"))
	}
}
