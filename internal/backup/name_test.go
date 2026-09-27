package backup

import (
	"testing"
	"time"
)

func TestFileName(t *testing.T) {
	ts := time.Date(2026, 9, 17, 14, 35, 5, 0, time.UTC)
	if got := FileName("Karot's Shop", ts, false); got != "karot-s-shop-20260917-143505.json.gz" {
		t.Fatalf("got %q", got)
	}
	if got := FileName("", ts, true); got != "pos-20260917-143505-partial.json.gz" {
		t.Fatalf("got %q", got)
	}
	if got := FileName("   ", ts, false); got != "pos-20260917-143505.json.gz" {
		t.Fatalf("blank-name fallback: got %q", got)
	}
}
