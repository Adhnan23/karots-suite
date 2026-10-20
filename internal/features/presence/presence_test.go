package presence

import (
	"testing"
	"time"
)

func TestStates(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	s.Beat(Beat{Device: "a", Name: "Zara"}, t0)
	s.Beat(Beat{Device: "b", Name: "amal", IdleSec: 180}, t0)
	s.Beat(Beat{Device: "c", Name: "Kamal", Locked: true, IdleSec: 900}, t0)
	s.Beat(Beat{Device: "d", Name: "Old"}, t0.Add(-2*time.Minute))
	s.Beat(Beat{Device: "e", Name: "Gone"}, t0.Add(-13*time.Hour))
	s.Beat(Beat{Device: "f", Name: "Left"}, t0)
	s.SignOut("f", t0.Add(-time.Second))
	s.SignOut("nobody", t0)
	s.Beat(Beat{Name: "no device"}, t0)

	got := s.List(t0)
	want := []struct{ name, state string }{{"amal", "idle"}, {"Kamal", "locked"}, {"Zara", "active"}, {"Left", "signedOut"}, {"Old", "away"}}
	if len(got) != len(want) {
		t.Fatalf("got %d rows: %+v", len(got), got)
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].State != w.state {
			t.Errorf("row %d = %s/%s, want %s/%s", i, got[i].Name, got[i].State, w.name, w.state)
		}
	}
	if len(s.m) != 5 {
		t.Errorf("the 13 h old device should be dropped, have %d", len(s.m))
	}
}

func TestLabel(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36":           "Windows · Chrome",
		"Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0":                                                "Linux · Firefox",
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36":              "Android · Chrome",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/1": "iPhone · Safari",
		"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/130.0 Safari/537.36 Edg/130.0":                                 "Windows · Edge",
		"curl/8": "",
	} {
		if got := label(ua); got != want {
			t.Errorf("label(%q) = %q, want %q", ua, got, want)
		}
	}
}
