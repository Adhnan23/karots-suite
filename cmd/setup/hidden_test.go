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
