package update

import (
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

// UPD-4: on Linux there is never a moment without a program in place during a
// swap or a rollback — a power cut at any instant leaves one that starts.
func TestUPD4_NeverWithoutAProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows can't replace a running .exe in one step; Recover covers it")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "karots-suite")
	os.WriteFile(exe, []byte("v0"), 0o755)
	var stop, missing atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		for !stop.Load() {
			if _, err := os.Stat(exe); err != nil {
				missing.Store(true)
			}
		}
	}()
	for i := range 300 {
		os.WriteFile(exe+".new", []byte{byte(i)}, 0o755)
		if err := Swap(exe, exe+".new"); err != nil {
			t.Fatal(err)
		}
		markPending(exe, "v")
		for range 3 {
			if _, err := BootGuard(exe, "v"); err != nil {
				t.Fatal(err)
			}
		}
	}
	stop.Store(true)
	<-done
	if missing.Load() {
		t.Fatal("UPD-4: for a moment there was no program in place")
	}
}

// UPD-4: a marker for a version that isn't the one running means the swap
// never happened (the power went between the two): the running program's
// boots are not counted against the update.
func TestUPD4_MarkerForAnotherVersionIgnored(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "karots-suite")
	os.WriteFile(exe, []byte("current"), 0o755)
	os.WriteFile(exe+".old", []byte("older"), 0o755)
	markPending(exe, "2026.11.30.0900")
	for range 4 {
		if rb, err := BootGuard(exe, "2026.11.20.0900"); rb || err != nil {
			t.Fatalf("UPD-4: the running version was rolled back for an update that never landed (%v %v)", rb, err)
		}
	}
	if read(t, exe) != "current" {
		t.Fatal("UPD-4: the running program was replaced")
	}
}

// UPD-4: a cut that left no program (Windows: between the two renames) is put
// right before the next start — the old program, else the new one.
func TestUPD4_RecoverPutsAProgramBack(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"swap cut", map[string]string{".old": "old", ".new": "new"}, "old"},
		{"rollback cut", map[string]string{".old": "old", ".failed": "bad"}, "old"},
		{"only new", map[string]string{".new": "new"}, "new"},
		{"in place", map[string]string{"": "cur", ".old": "old"}, "cur"},
	} {
		dir := t.TempDir()
		exe := filepath.Join(dir, "karots-suite")
		for suf, body := range c.files {
			os.WriteFile(exe+suf, []byte(body), 0o755)
		}
		Recover(exe)
		if got := read(t, exe); got != c.want {
			t.Errorf("%s: %q in place, want %q", c.name, got, c.want)
		}
	}
}
