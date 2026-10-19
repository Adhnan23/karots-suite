package main

import (
	"errors"
	"slices"
	"testing"
	"time"
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
	defer func(d time.Duration) { crashDelay = d }(crashDelay)
	crashDelay = 0
	run := func(codes ...int) (int, int) {
		calls := 0
		got := runChild(func() (int, error) { c := codes[calls]; calls++; return c, nil })
		return got, calls
	}
	if got, calls := run(75, 75, 0); got != 0 || calls != 3 {
		t.Fatalf("after an update: code %d, %d starts; want 0, 3", got, calls)
	}
	// A crash is started again too (the scheduled task wouldn't), so BootGuard
	// can count failed starts and roll a bad update back.
	if got, calls := run(1, 2, 0); got != 0 || calls != 3 {
		t.Fatalf("after crashes: code %d, %d starts; want 0, 3", got, calls)
	}
	calls := 0
	if got := runChild(func() (int, error) { calls++; return 2, errors.New("can't start") }); got != 2 || calls != 1 {
		t.Fatalf("a program that can't start: code %d, %d starts; want 2, 1", got, calls)
	}
}
