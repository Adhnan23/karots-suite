package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"
)

func TestLoginSetsStrictHttpOnlyCookieAndMeWorks(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009001", "cashier")
	rec := te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009001", Pin: "1234"}, nil)
	if rec.Code != 200 {
		t.Fatalf("login %d %s", rec.Code, rec.Body)
	}
	ck := sessionCookie(t, rec)
	if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie not HttpOnly+Strict: %+v", ck)
	}
	rec = te.do(http.MethodGet, "/api/v1/me", nil, ck)
	var me Me
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck
	if rec.Code != 200 || me.User.Role != "cashier" || me.Flags.CanExitKiosk || me.Plugins == nil {
		t.Fatalf("me %d %+v", rec.Code, me)
	}
}

func TestLoginWrongPin(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009002", "cashier")
	rec := te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009002", Pin: "9999"}, nil)
	if rec.Code < 400 || rec.Code >= 500 || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("wrong pin gave %d cookies=%v", rec.Code, rec.Result().Cookies())
	}
}

func TestMeWithoutCookieIs401(t *testing.T) {
	te := newTestEnv(t)
	if rec := te.do(http.MethodGet, "/api/v1/me", nil, nil); rec.Code != 401 {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestLockThenMeStillWorksThenUnlock(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009003", "cashier")
	ck := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009003", Pin: "1234"}, nil))
	locked := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/lock", nil, ck))
	rec := te.do(http.MethodGet, "/api/v1/me", nil, locked)
	var me Me
	json.Unmarshal(rec.Body.Bytes(), &me) //nolint:errcheck
	if rec.Code != 200 || !me.Locked {
		t.Fatalf("locked me %d %+v", rec.Code, me)
	}
	if rec := te.do(http.MethodPost, "/api/v1/auth/pin", ChangePinRequest{"1234", "5678", "5678"}, locked); errCode(rec) != "SESSION_LOCKED" {
		t.Fatalf("locked pin change: %d %s", rec.Code, rec.Body)
	}
	un := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/unlock", LoginRequest{Phone: "0770009003", Pin: "1234"}, locked))
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/me", nil, un).Body.Bytes(), &me) //nolint:errcheck
	if me.Locked {
		t.Fatal("still locked after unlock")
	}
}

func TestLoginInfoPublic(t *testing.T) {
	te := newTestEnv(t)
	rec := te.do(http.MethodGet, "/api/v1/auth/login-info", nil, nil)
	var li LoginInfo
	json.Unmarshal(rec.Body.Bytes(), &li) //nolint:errcheck
	if rec.Code != 200 || !regexp.MustCompile(`^[0-9A-F]{8}$`).MatchString(li.InstallID) || li.ServerTime == "" {
		t.Fatalf("%d %+v", rec.Code, li)
	}
}

func TestUnknownAPIPathIsJSON404(t *testing.T) {
	te := newTestEnv(t)
	rec := te.do(http.MethodGet, "/api/v1/nope", nil, nil)
	if rec.Code != 404 || errCode(rec) == "" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009004", "admin")
	ck := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009004", Pin: "1234"}, nil))
	rec := te.do(http.MethodPost, "/api/v1/auth/logout", nil, ck)
	if rec.Code != 204 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == ck.Name && c.MaxAge >= 0 {
			t.Fatal("cookie not cleared")
		}
	}
}
