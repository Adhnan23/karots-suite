package pocketcashier

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

func partsFix(t *testing.T) (*env, string) {
	en := newEnv(t)
	t.Cleanup(plugin.ResetOfflineDataForTest)
	secret, _ := en.activePhone()
	return en, secret
}

func TestPartsHoldsWhatPluginsDeclare(t *testing.T) {
	en, secret := partsFix(t)
	(&plugin.Registry{}).AddOfflineData(plugin.OfflineData{Key: "t1", Build: func(context.Context) (any, error) { return map[string]any{"n": 1}, nil }})
	rec := en.do(http.MethodGet, "/parts", nil, nil, secret)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"t1":{"n":1}`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestPartsNotModified(t *testing.T) {
	en, secret := partsFix(t)
	n := 1
	(&plugin.Registry{}).AddOfflineData(plugin.OfflineData{Key: "t1", Build: func(context.Context) (any, error) { return n, nil }})
	first := en.do(http.MethodGet, "/parts", nil, nil, secret)
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	req := en.doHeader(http.MethodGet, "/parts", secret, "If-None-Match", tag)
	if req.Code != http.StatusNotModified {
		t.Fatalf("same data: %d", req.Code)
	}
	n = 2
	if req := en.doHeader(http.MethodGet, "/parts", secret, "If-None-Match", tag); req.Code != http.StatusOK {
		t.Fatalf("changed data: %d", req.Code)
	}
}

func TestPartsNeedPhoneKey(t *testing.T) {
	en, _ := partsFix(t)
	if rec := en.do(http.MethodGet, "/parts", nil, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestPartFailingFailsTheCall(t *testing.T) {
	en, secret := partsFix(t)
	(&plugin.Registry{}).AddOfflineData(plugin.OfflineData{Key: "bad", Build: func(context.Context) (any, error) { return nil, errors.New("boom") }})
	if rec := en.do(http.MethodGet, "/parts", nil, nil, secret); rec.Code != http.StatusInternalServerError {
		t.Fatalf("the phone must keep its last good parts: %d", rec.Code)
	}
}
