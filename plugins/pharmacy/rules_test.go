package pharmacy

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestDosesPerDay(t *testing.T) {
	for in, want := range map[string]string{
		"1-0-1": "2", "1-1-1": "3", "0-0-1": "1", "1/2-0-1/2": "1", "½-0-½": "1", "2-0-2": "4",
		"1-1-1-1": "4", " 1 - 0 - 1 ": "2", "BD": "2", "tds": "3", "od": "1", "nocte": "1", "qid": "4",
		"1 tablet twice daily": "0", "": "0", "1-0": "0", "a-b-c": "0",
	} {
		if got := DosesPerDay(in); got.String() != want {
			t.Errorf("DosesPerDay(%q) = %s, want %s", in, got, want)
		}
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// karots-pos rules: last hand-over + days covered; a repeat shows from 5 days
// before run-out and stays, a one-off until 30 days past it.
func TestNextRefillDue(t *testing.T) {
	at := day(2026, 9, 1)
	if _, ok := nextRefillDue(nil, 30); ok {
		t.Error("never dispensed → no due date")
	}
	if _, ok := nextRefillDue(&at, 0); ok {
		t.Error("no days supplied → no due date")
	}
	if due, ok := nextRefillDue(&at, 30); !ok || !due.Equal(day(2026, 10, 1)) {
		t.Errorf("30 days from 1 Sep → 1 Oct, got %v", due)
	}
}

func TestRefillIsDue(t *testing.T) {
	due := day(2026, 10, 1)
	for _, c := range []struct {
		repeat bool
		now    time.Time
		want   bool
	}{
		{true, day(2026, 9, 20), false}, {true, day(2026, 9, 28), true}, {true, day(2026, 12, 1), true},
		{false, day(2026, 10, 15), true}, {false, day(2026, 12, 1), false},
	} {
		if got := refillIsDue(due, c.repeat, c.now); got != c.want {
			t.Errorf("repeat=%v now=%v: %v, want %v", c.repeat, c.now, got, c.want)
		}
	}
}

func TestDueStatus(t *testing.T) {
	due := day(2026, 10, 1)
	for now, want := range map[time.Time]string{
		day(2026, 9, 1): "covered", day(2026, 9, 27): "soon", day(2026, 10, 2): "overdue",
	} {
		if got := dueStatus(&due, now); got != want {
			t.Errorf("%v: %s, want %s", now, got, want)
		}
	}
	if dueStatus(nil, due) != "" {
		t.Error("no due date → no badge")
	}
}

// What the Add button puts in the cart: the rest of a one-off; a repeat starts
// a fresh fill once the last one is complete.
func TestOwed(t *testing.T) {
	d := decimal.NewFromInt
	for _, c := range []struct {
		qty, done int64
		repeat    bool
		want      int64
	}{
		{60, 0, false, 60}, {60, 20, false, 40}, {60, 60, false, 0}, {60, 80, false, 0},
		{60, 60, true, 60}, {60, 90, true, 30}, {0, 10, true, 0},
	} {
		if got := owed(d(c.qty), d(c.done), c.repeat); !got.Equal(d(c.want)) {
			t.Errorf("owed(%d, %d, %v) = %s, want %d", c.qty, c.done, c.repeat, got, c.want)
		}
	}
}

func TestSupplyDays(t *testing.T) {
	d := decimal.NewFromInt
	for _, c := range []struct{ tablets, perDay, want int64 }{{60, 2, 30}, {14, 3, 4}, {10, 0, 0}, {1, 2, 0}} {
		if got := supplyDays(d(c.tablets), d(c.perDay)); got != int(c.want) {
			t.Errorf("supplyDays(%d, %d) = %d, want %d", c.tablets, c.perDay, got, c.want)
		}
	}
}
