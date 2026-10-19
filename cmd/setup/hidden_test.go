package main

import (
	"slices"
	"testing"
)

func TestParseHidden(t *testing.T) {
	h, err := parseHidden([]string{"-log", `C:\k\logs\karots.log`, "karots-suite.exe", "-x", "y"})
	if err != nil || h.log != `C:\k\logs\karots.log` || h.prog != "karots-suite.exe" || !slices.Equal(h.args, []string{"-x", "y"}) {
		t.Fatalf("%+v %v", h, err)
	}
	if h, err := parseHidden([]string{"cmd.exe", "/c", "kiosk.cmd"}); err != nil || h.log != "" || h.prog != "cmd.exe" {
		t.Fatalf("no log: %+v %v", h, err)
	}
	if _, err := parseHidden(nil); err == nil {
		t.Fatal("nothing to run accepted")
	}
}

func TestRunChildRestarts(t *testing.T) {
	codes := []int{75, 75, 0}
	calls := 0
	got := runChild(func() (int, error) { c := codes[calls]; calls++; return c, nil })
	if got != 0 || calls != 3 {
		t.Fatalf("after an update: code %d, %d starts; want 0, 3", got, calls)
	}
	calls = 0
	if got := runChild(func() (int, error) { calls++; return 1, nil }); got != 1 || calls != 1 {
		t.Fatalf("a crash: code %d, %d starts; want 1, 1 (the task's restart-on-failure handles it)", got, calls)
	}
}
