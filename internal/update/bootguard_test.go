package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestBootGuard(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "karots-suite")
	os.WriteFile(exe, []byte("old"), 0o755)

	if rb, err := BootGuard(exe); rb || err != nil {
		t.Fatalf("no marker: %v %v", rb, err)
	}

	os.WriteFile(exe+".new", []byte("new"), 0o755)
	if err := Swap(exe, exe+".new"); err != nil {
		t.Fatal(err)
	}
	if err := markPending(exe, "2026.10.19.1530"); err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{false, false, true} {
		rb, err := BootGuard(exe)
		if err != nil || rb != want {
			t.Fatalf("boot %d: rolledBack=%v err=%v, want %v", i+1, rb, err, want)
		}
	}
	if read(t, exe) != "old" {
		t.Fatal("rollback should put the old binary back")
	}
	if read(t, exe+".rolledback") != "2026.10.19.1530" {
		t.Fatal(".rolledback should name the version that couldn't start")
	}

	u := &Updater{Current: "2026.10.18.0900", Exe: exe}
	if s := u.Status(); s.RolledBackFrom != "2026.10.19.1530" {
		t.Fatalf("RolledBackFrom = %q", s.RolledBackFrom)
	}
	u.Dismiss()
	if s := u.Status(); s.RolledBackFrom != "" {
		t.Fatalf("after Dismiss RolledBackFrom = %q", s.RolledBackFrom)
	}
}

func TestMarkHealthy(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "karots-suite")
	os.WriteFile(exe, []byte("new"), 0o755)
	os.WriteFile(exe+".old", []byte("old"), 0o755)
	markPending(exe, "2026.10.19.1530")
	BootGuard(exe)
	MarkHealthy(exe, "2026.10.19.1530")
	for range 3 {
		if rb, _ := BootGuard(exe); rb {
			t.Fatal("a healthy binary must never be rolled back")
		}
	}
	if read(t, exe) != "new" {
		t.Fatal("exe changed")
	}
}

// The version BootGuard gave up on is never offered again.
func TestRolledBackNotOffered(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	bad := signed(t, priv, "2026.10.19.1530", "aa")
	srv := feed(t, &bad)
	exe := filepath.Join(t.TempDir(), "karots-suite")
	os.WriteFile(exe+".rolledback", []byte("2026.10.19.1530"), 0o644)
	u := &Updater{Current: "2026.10.18.0900", BaseURL: srv.URL, Channel: testChannel, PubKey: pub, Exe: exe}
	u.Check(context.Background())
	if s := u.Status(); s.State != StateUpToDate || s.Latest != "" {
		t.Fatalf("status %+v", s)
	}
	u.Dismiss() // the notice goes; the bad version stays skipped
	u.Check(context.Background())
	if s := u.Status(); s.State != StateUpToDate || s.RolledBackFrom != "" {
		t.Fatalf("after dismiss: %+v", s)
	}
}

// The old binary's "stayed up" timer must not clear a fresh update's marker.
func TestMarkHealthyOnlyItsOwnVersion(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "karots-suite")
	markPending(exe, "2026.10.19.1530")
	MarkHealthy(exe, "2026.10.18.0900") // the old binary, 2 minutes after it booted
	if _, err := os.Stat(markerPath(exe)); err != nil {
		t.Fatal("the new version's marker was removed")
	}
	MarkHealthy(exe, "2026.10.19.1530")
	if _, err := os.Stat(markerPath(exe)); err == nil {
		t.Fatal("the marker should be gone once the new version stayed up")
	}
}
