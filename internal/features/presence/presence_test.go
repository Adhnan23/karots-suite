package presence

import (
	"testing"
	"time"
)

func TestStates(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	s.Beat(Beat{Device: "a", UserID: 1, Name: "Zara"}, t0)
	s.Beat(Beat{Device: "b", UserID: 2, Name: "amal", IdleSec: 180}, t0)
	s.Beat(Beat{Device: "c", UserID: 3, Name: "Kamal", Locked: true, IdleSec: 900}, t0)
	s.Beat(Beat{Device: "d", UserID: 4, Name: "Old"}, t0.Add(-2*time.Minute))
	s.Beat(Beat{Device: "e", UserID: 5, Name: "Gone"}, t0.Add(-13*time.Hour))
	s.Beat(Beat{Device: "f", UserID: 6, Name: "Left"}, t0)
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

func TestSameMachineNewID(t *testing.T) {
	s := New()
	t0 := time.Now()
	s.Beat(Beat{Device: "old", UserID: 1, IP: "10.0.0.5", UA: "X"}, t0)
	s.Beat(Beat{Device: "other-user", UserID: 2, Name: "B", IP: "10.0.0.5", UA: "X"}, t0)
	s.Beat(Beat{Device: "phone", UserID: 1, IP: "10.0.0.9", UA: "X"}, t0)
	s.Beat(Beat{Device: "new", UserID: 1, IP: "10.0.0.5", UA: "X"}, t0)
	s.Beat(Beat{Device: "phone", UserID: 1, Name: "A", IP: "10.0.0.9", UA: "X", IdleSec: 300}, t0)
	s.Beat(Beat{Device: "new", UserID: 1, Name: "A", IP: "10.0.0.5", UA: "X"}, t0)
	if l := s.List(t0); l[0].Device != "new" {
		t.Errorf("the active device should come first, got %s", l[0].Device)
	}
	if _, ok := s.m["old"]; ok || len(s.m) != 3 {
		t.Fatalf("old id should be replaced by new, have %v", s.m)
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

func TestRemoteLockAndCart(t *testing.T) {
	s := New()
	t0 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	cart := Beat{Device: "till", UserID: 2, Name: "Cashier", CartItems: 1, CartTotal: "50.00"}
	s.Beat(cart, t0)
	if _, err := s.Lock("till", t0.Add(time.Minute)); err != ErrServing {
		t.Fatalf("a cart changed a minute ago is a sale: got %v", err)
	}
	// The same item parked for 12 min, the mouse still moving: not serving, flagged.
	for m := 1; m <= 12; m++ {
		s.Beat(cart, t0.Add(time.Duration(m)*time.Minute))
	}
	r := s.List(t0.Add(12 * time.Minute))[0]
	if r.Serving || r.UntouchedMin != 12 || r.State != "active" {
		t.Fatalf("parked cart: %+v", r)
	}
	if _, err := s.Lock("till", t0.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !s.Beat(cart, t0.Add(12*time.Minute+20*time.Second)) {
		t.Fatal("the next beat should say lock")
	}
	locked := cart
	locked.Locked = true
	if s.Beat(locked, t0.Add(13*time.Minute)) {
		t.Fatal("a locked beat clears the ask")
	}
	if _, err := s.Lock("till", t0.Add(13*time.Minute)); err != ErrNotOpen {
		t.Fatalf("already locked: %v", err)
	}
	if _, err := s.Lock("nope", t0); err != ErrGone {
		t.Fatalf("unknown device: %v", err)
	}
	// Adding an item restarts the clock.
	more := cart
	more.CartItems, more.CartTotal = 2, "100.00"
	s.Beat(more, t0.Add(14*time.Minute))
	if r := s.List(t0.Add(15 * time.Minute))[0]; !r.Serving || r.UntouchedMin != 0 {
		t.Fatalf("changed cart: %+v", r)
	}
}
