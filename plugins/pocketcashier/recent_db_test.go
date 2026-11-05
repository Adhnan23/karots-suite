package pocketcashier

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRecentNeedsPhoneKey(t *testing.T) {
	en := newEnv(t)
	if rec := en.do(http.MethodGet, "/recent", nil, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: %d %s", rec.Code, rec.Body)
	}
	var after int64
	var since time.Time
	var lastAt string
	en.p.core.RecentSales = func(_ context.Context, a int64, sn time.Time, at string, _ int) (any, error) {
		after, since, lastAt = a, sn, at
		return map[string]any{"sales": []any{}, "nextSince": "x"}, nil
	}
	secret, _ := en.activePhone()
	rec := en.do(http.MethodGet, "/recent", nil, nil, secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("paired phone: %d %s", rec.Code, rec.Body)
	}
	if rec := en.do(http.MethodGet, "/recent?after=7&since=2026-11-05T10:00:00.5Z&at=stamp", nil, nil, secret); rec.Code != http.StatusOK || after != 7 || since.IsZero() || lastAt != "stamp" {
		t.Fatalf("cursor not passed: %d after=%d since=%v", rec.Code, after, since)
	}
	if rec := en.do(http.MethodGet, "/recent?after=x", nil, nil, secret); rec.Code != http.StatusBadRequest {
		t.Fatalf("a bad cursor: %d", rec.Code)
	}
	got := decodeAs[struct {
		Sales     []any  `json:"sales"`
		NextSince string `json:"nextSince"`
	}](t, rec)
	if got.NextSince == "" || got.Sales == nil {
		t.Fatalf("reply: %s", rec.Body)
	}
}
