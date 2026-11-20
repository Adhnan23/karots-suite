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

// RPT-11: the presets are whole shop days, weeks run Monday–Sunday, and the
// dates shown are the first and last day (to inclusive). Expected values are
// worked out here from the calendar, not from the code.
func TestPresetsAreWholeWeeksAndMonths(t *testing.T) {
	now := time.Now().In(datetime.Location)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, datetime.Location)
	monday := day
	for monday.Weekday() != time.Monday {
		monday = monday.AddDate(0, 0, -1)
	}
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, datetime.Location)
	for _, c := range []struct {
		preset   string
		from, to time.Time
	}{
		{"today", day, day.AddDate(0, 0, 1)},
		{"this-week", monday, monday.AddDate(0, 0, 7)},
		{"last-week", monday.AddDate(0, 0, -7), monday},
		{"this-month", month, month.AddDate(0, 1, 0)},
		{"last-month", month.AddDate(0, -1, 0), month},
		{"this-year", time.Date(now.Year(), 1, 1, 0, 0, 0, 0, datetime.Location), time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, datetime.Location)},
	} {
		from, to, fs, ts, err := ResolveRange(c.preset, "", "")
		mustR(t, err)
		if !from.Equal(c.from) || !to.Equal(c.to) {
			t.Errorf("%s = [%s, %s), want [%s, %s)", c.preset, from, to, c.from, c.to)
		}
		if fs != c.from.Format("2006-01-02") || ts != c.to.AddDate(0, 0, -1).Format("2006-01-02") {
			t.Errorf("%s shows %s – %s, want its first and last day", c.preset, fs, ts)
		}
	}
	if _, _, _, _, err := ResolveRange("next-week", "", ""); err == nil {
		t.Error("an unknown preset was taken")
	}
	// No dates: this month, shown as its first and last day; a bad date is refused.
	from, to, fs, ts, err := ResolveRange("", "", "")
	mustR(t, err)
	if !from.Equal(month) || !to.Equal(month.AddDate(0, 1, 0)) || fs != month.Format("2006-01-02") ||
		ts != month.AddDate(0, 1, -1).Format("2006-01-02") {
		t.Errorf("no dates = [%s, %s) shown %s – %s, want this month", from, to, fs, ts)
	}
	if _, _, _, _, err := ResolveRange("", "7 Oct", ""); err == nil {
		t.Error("a bad from date was taken")
	}
	if _, _, _, _, err := ResolveRange("", "", "2026-13-01"); err == nil {
		t.Error("a bad to date was taken")
	}
}

// Reorder demand for no products asks the database nothing.
func TestProductQtySoldOfNothing(t *testing.T) {
	got, err := (&Service{}).ProductQtySold(t.Context(), nil, time.Now(), time.Now())
	if err != nil || len(got) != 0 {
		t.Fatalf("%v, %v", got, err)
	}
}
