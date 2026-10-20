package cashregister

import (
	"testing"
	"time"

	"karots-suite/internal/datetime"
)

func TestStale(t *testing.T) {
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, datetime.Location) }
	for _, c := range []struct {
		opened, now time.Time
		want        bool
	}{
		{at(20, 8), at(20, 22), false}, // same day, however long
		{at(19, 18), at(20, 8), true},  // yesterday evening, next morning
		{at(19, 22), at(20, 1), false}, // past midnight, only 3 h: a late shop still trading
		{at(19, 22), at(20, 4), true},  // 6 h
		{at(17, 9), at(20, 9), true},
	} {
		if got := Stale(c.opened, c.now); got != c.want {
			t.Errorf("Stale(%v, %v) = %v, want %v", c.opened, c.now, got, c.want)
		}
	}
	closed := at(20, 9)
	if !ClosedLate(at(19, 18), &closed) || ClosedLate(at(20, 8), &closed) || ClosedLate(at(19, 8), nil) {
		t.Error("ClosedLate")
	}
}
