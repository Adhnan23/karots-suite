package battle

import (
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/ledgercheck/ltest"
)

// agree holds every report over [from, to] to the raw rows and to each other
// (area 7's auditor, ltest.Agree), read as the shop's admin.
func (s *shop) agree(t ltest.T, from, to string) {
	helperOf(t)
	ltest.Agree(t, s.db, func(path string, out any) error { return s.admin.Do("GET", path, nil, out) }, from, to)
}

// helperOf marks t's caller a helper when t can (testing.T can; ltest.T may not).
func helperOf(t ltest.T) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
}

// agreeToday runs agree over today (the machines' shop lives in one day).
func (s *shop) agreeToday(t ltest.T) {
	helperOf(t)
	today := time.Now().In(datetime.Location).Format("2006-01-02")
	s.agree(t, today, today)
}

// agreeSometimes runs agreeToday every 25th machine check: every step would add
// half an hour to a long run; each run also ends with one (area 7, Task 5).
func (s *shop) agreeSometimes(t ltest.T) {
	if s.checks++; s.checks%25 == 0 {
		s.agreeToday(t)
	}
}
