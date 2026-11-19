package config

import (
	"testing"
	"time"
)

// A zero or negative duration (BACKUP_INTERVAL=0) crashed the backup ticker a
// moment after every start; it means "not set", so the default applies.
func TestDurationNeverZero(t *testing.T) {
	for _, v := range []string{"0", "0s", "-1h"} {
		t.Setenv("BACKUP_INTERVAL", v)
		if got := mustDuration("BACKUP_INTERVAL", 6*time.Hour); got != 6*time.Hour {
			t.Errorf("BACKUP_INTERVAL=%s gave %s, want the 6h default", v, got)
		}
	}
	t.Setenv("BACKUP_INTERVAL", "90m")
	if got := mustDuration("BACKUP_INTERVAL", 6*time.Hour); got != 90*time.Minute {
		t.Errorf("90m gave %s", got)
	}
}
