package pocketcashier

import "testing"

// PKT-16: the port is read when asked, not when the package loads, so a value
// LoadDotEnv puts in the environment after start-up still counts.
func TestPortReadAfterStart(t *testing.T) {
	t.Setenv("KAROTS_POCKET_PORT", "")
	if got := Port(); got != 8443 {
		t.Fatalf("default port = %d, want 8443", got)
	}
	t.Setenv("KAROTS_POCKET_PORT", "9443")
	if got := Port(); got != 9443 {
		t.Fatalf("port after .env = %d, want 9443", got)
	}
	t.Setenv("KAROTS_POCKET_PORT", "nope")
	if got := Port(); got != 8443 {
		t.Fatalf("bad value port = %d, want 8443", got)
	}
}
