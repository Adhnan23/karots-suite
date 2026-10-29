package pharmacy

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Refill rules: a repeat script shows from leadDays before a drug runs
// out and stays listed; a one-off until graceDays past run-out. A drug the shop
// phoned about drops off the call list for calledDays.
const (
	leadDays   = 5
	graceDays  = 30
	calledDays = 3
)

// doseWords are the Latin shorthands doctors write beside (or instead of) 1-0-1.
var doseWords = map[string]int64{"od": 1, "mane": 1, "nocte": 1, "bd": 2, "bid": 2, "tds": 3, "tid": 3, "qds": 4, "qid": 4}

// DosesPerDay reads a Sri Lankan dosage — morning-noon-night counts such as 1-0-1
// (2 a day), 1/2-0-1/2 (1) or 1-1-1-1 (4), or od / bd / tds / qid — as doses a
// day. 0 when it can't tell (the pharmacist types it).
func DosesPerDay(dosage string) decimal.Decimal {
	s := strings.ToLower(strings.TrimSpace(dosage))
	if n, ok := doseWords[s]; ok {
		return decimal.NewFromInt(n)
	}
	parts := strings.Split(strings.ReplaceAll(s, "½", "1/2"), "-")
	if len(parts) < 3 || len(parts) > 4 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, p := range parts {
		v, ok := doseAmount(strings.TrimSpace(p))
		if !ok {
			return decimal.Zero
		}
		sum = sum.Add(v)
	}
	return sum
}

// doseAmount parses "1", "2", "1/2", "0.5".
func doseAmount(p string) (decimal.Decimal, bool) {
	if a, b, ok := strings.Cut(p, "/"); ok {
		n, e1 := decimal.NewFromString(a)
		d, e2 := decimal.NewFromString(b)
		if e1 != nil || e2 != nil || !d.IsPositive() {
			return decimal.Zero, false
		}
		return n.Div(d), true
	}
	v, err := decimal.NewFromString(p)
	return v, err == nil && !v.IsNegative()
}

// supplyDays is how many whole days tablets last at perDay (0 = not tracked).
func supplyDays(tablets, perDay decimal.Decimal) int {
	if !perDay.IsPositive() || !tablets.IsPositive() {
		return 0
	}
	return int(tablets.Div(perDay).IntPart())
}

// nextRefillDue: last hand-over + the days it covered; none without either.
func nextRefillDue(lastAt *time.Time, days int) (time.Time, bool) {
	if lastAt == nil || days <= 0 {
		return time.Time{}, false
	}
	return lastAt.AddDate(0, 0, days), true
}

// refillIsDue puts a drug on the call list (see leadDays / graceDays).
func refillIsDue(due time.Time, repeat bool, now time.Time) bool {
	if now.Before(due.AddDate(0, 0, -leadDays)) {
		return false
	}
	return repeat || !now.After(due.AddDate(0, 0, graceDays))
}

// dueStatus is a drug's badge: overdue, soon (inside the lead window), covered, or "".
func dueStatus(due *time.Time, now time.Time) string {
	switch {
	case due == nil:
		return ""
	case now.After(*due):
		return "overdue"
	case !now.Before(due.AddDate(0, 0, -leadDays)):
		return "soon"
	}
	return "covered"
}

// owed is what Add puts in the cart: the rest of a one-off; for a repeat, a
// full fill every time (the patient is back because the last one ran out).
func owed(qty, dispensed decimal.Decimal, repeat bool) decimal.Decimal {
	if !qty.IsPositive() {
		return decimal.Zero
	}
	if repeat {
		return qty
	}
	return decimal.Max(qty.Sub(dispensed), decimal.Zero)
}
