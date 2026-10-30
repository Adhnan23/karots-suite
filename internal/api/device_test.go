package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/plugin"
)

// on sends a JSON request from one device (X-Ks-Device).
func (te *testEnv) on(dev, method, path string, body any, ck *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(deviceHeader, dev)
	if ck != nil {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func devPhone() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }

// signIn: a user signs in on a device, labelled.
func (te *testEnv) signIn(t *testing.T, dev, label, phone string) *http.Cookie {
	t.Helper()
	return sessionCookie(t, te.on(dev, http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "1234", DeviceLabel: label}, nil))
}

func oneDevice(t *testing.T, guards ...plugin.TakeoverGuard) {
	plugin.SetOneDeviceForTest(true, guards...)
	t.Cleanup(func() { plugin.SetOneDeviceForTest(false) })
}

func TestOneDeviceOffByDefault(t *testing.T) {
	te := newTestEnv(t)
	ph := devPhone()
	te.seedUser(t, ph, "cashier")
	a := te.signIn(t, "dev-A", "Counter PC", ph)
	b := te.signIn(t, "dev-B", "Phone 2", ph)
	if r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, a); r.Code != http.StatusOK {
		t.Fatalf("A: %d", r.Code)
	}
	if r := te.on("dev-B", http.MethodGet, "/api/v1/till/config", nil, b); r.Code != http.StatusOK {
		t.Fatalf("B: %d", r.Code)
	}
}

func TestTakeover(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t)
	ph := devPhone()
	te.seedUser(t, ph, "cashier")
	a := te.signIn(t, "dev-A", "Counter PC", ph)
	b := te.signIn(t, "dev-B", "Phone 2", ph)
	r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, a)
	if r.Code != http.StatusForbidden || errCode(r) != "SESSION_LOCKED" || !strings.Contains(r.Body.String(), "Phone 2") {
		t.Fatalf("A after takeover: %d %s", r.Code, r.Body)
	}
	if me := decode[Me](t, te.on("dev-A", http.MethodGet, "/api/v1/me", nil, a)); me.MovedTo != "Phone 2" {
		t.Fatalf("movedTo %q", me.MovedTo)
	}
	if rep := decode[PresenceReply](t, te.on("dev-A", http.MethodPost, "/api/v1/presence", PresenceBeat{Path: "/till"}, a)); !rep.Moved {
		t.Fatal("presence should say moved")
	}
	hold := te.on("dev-A", http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "Moved to another device", SaleType: "retail", Cart: `{"lines":[]}`, ItemCount: 1, Total: "10"}, a)
	if hold.Code >= 300 {
		t.Fatalf("A can't park its cart: %d %s", hold.Code, hold.Body)
	}
	if r := te.on("dev-B", http.MethodGet, "/api/v1/till/config", nil, b); r.Code != http.StatusOK {
		t.Fatalf("B: %d", r.Code)
	}
	te.db.Exec(`DELETE FROM held_sales WHERE label = 'Moved to another device'`) //nolint:errcheck
}

func TestUnlockTakesBack(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t)
	ph := devPhone()
	te.seedUser(t, ph, "cashier")
	a := te.signIn(t, "dev-A", "Counter PC", ph)
	b := te.signIn(t, "dev-B", "Phone 2", ph)
	if r := te.on("dev-A", http.MethodPost, "/api/v1/auth/unlock", LoginRequest{Phone: ph, Pin: "1234"}, a); r.Code != http.StatusNoContent {
		t.Fatalf("unlock: %d %s", r.Code, r.Body)
	}
	if r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, a); r.Code != http.StatusOK {
		t.Fatalf("A after taking back: %d", r.Code)
	}
	if r := te.on("dev-B", http.MethodGet, "/api/v1/till/config", nil, b); r.Code != http.StatusForbidden {
		t.Fatalf("B should be locked now: %d", r.Code)
	}
}

func TestSystemExempt(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t)
	ph := devPhone()
	id := te.seedUser(t, ph, "admin")
	te.db.MustExec(`UPDATE users SET is_system = true WHERE id = $1`, id)
	a := te.signIn(t, "dev-A", "Counter PC", ph)
	te.signIn(t, "dev-B", "Phone 2", ph)
	if r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, a); r.Code != http.StatusOK {
		t.Fatalf("system user locked out: %d %s", r.Code, r.Body)
	}
}

func TestTakeoverGuardBlocks(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t, func(context.Context, int64, string) (bool, string) { return true, "on Phone 2" })
	ph := devPhone()
	te.seedUser(t, ph, "cashier")
	a := te.signIn(t, "dev-A", "Counter PC", ph)
	r := te.on("dev-B", http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: ph, Pin: "1234"}, nil)
	if r.Code != http.StatusConflict || errCode(r) != "DEVICE_HELD" || !strings.Contains(r.Body.String(), "on Phone 2") {
		t.Fatalf("B login: %d %s", r.Code, r.Body)
	}
	if r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, a); r.Code != http.StatusOK {
		t.Fatalf("A should keep working: %d", r.Code)
	}
}
