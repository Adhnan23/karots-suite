package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (te *testEnv) beat(dev string, body PresenceBeat, ck *http.Cookie) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/presence", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(deviceHeader, dev)
	req.AddCookie(ck)
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func TestPresenceLiveUsers(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009101", "admin")
	te.seedUser(t, "0770009102", "cashier")
	te.seedUser(t, "0770009103", "manager")
	admin := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009101", Pin: "1234"}, nil))
	cashier := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009102", Pin: "1234"}, nil))
	manager := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: "0770009103", Pin: "1234"}, nil))

	if rec := te.beat("pc-1", PresenceBeat{Path: "/till", CartItems: 3, CartTotal: "1250.00"}, cashier); rec.Code != 204 {
		t.Fatalf("beat %d %s", rec.Code, rec.Body)
	}
	locked := sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/lock", nil, cashier))
	if rec := te.beat("pc-1", PresenceBeat{Path: "/lock", Note: "Playing Blocks"}, locked); rec.Code != 204 {
		t.Fatalf("locked beat %d", rec.Code)
	}
	list := func() []PresenceRow {
		rec := te.do(http.MethodGet, "/api/v1/admin/presence", nil, admin)
		if rec.Code != 200 {
			t.Fatalf("presence %d %s", rec.Code, rec.Body)
		}
		var rows []PresenceRow
		json.Unmarshal(rec.Body.Bytes(), &rows) //nolint:errcheck
		return rows
	}
	rows := list()
	if len(rows) != 1 || rows[0].State != "locked" || rows[0].Path != "/lock" || rows[0].Note != "Playing Blocks" || rows[0].Role != "cashier" || rows[0].TillOpened != nil {
		t.Fatalf("rows %+v", rows)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/presence", nil, manager); rec.Code != 403 {
		t.Fatalf("manager got %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/presence", PresenceBeat{}, nil); rec.Code != 401 {
		t.Fatalf("no cookie got %d", rec.Code)
	}

	// Sign out on that device marks it signed out at once.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set(deviceHeader, "pc-1")
	req.AddCookie(locked)
	te.e.ServeHTTP(httptest.NewRecorder(), req)
	if rows = list(); len(rows) != 1 || rows[0].State != "signedOut" {
		t.Fatalf("after sign out %+v", rows)
	}
}
