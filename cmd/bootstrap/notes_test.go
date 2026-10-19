package main

import "testing"

func TestFriendlyNotes(t *testing.T) {
	got := friendlyNotes([]string{
		"fix(backup): restore works on installed shops — it set session_replication_role, …",
		"feat: software update card",
		"fix(recharge): reload reversal keeps the float", // plugin this shop lacks
		"fix(pharmacy): refill dates; were off by one",
		"docs: M4 notes",
		"test(money): flows",
		"Merge branch 'x'",
	}, []string{"pharmacy"}, []string{"pharmacy", "recharge"})
	want := "• Restore works on installed shops\n• Software update card\n• Refill dates"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
