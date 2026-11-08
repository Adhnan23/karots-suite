package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// shelf sends a request as Karots Shelf would: its device and the X-Ks-App header.
func (te *testEnv) shelf(dev, method, path string, body any, ck *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(deviceHeader, dev)
	req.Header.Set(shelfHeader, shelfApp)
	if ck != nil {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func (te *testEnv) shelfCashier(t *testing.T, ticked bool) string {
	t.Helper()
	ph := devPhone()
	id := te.seedUser(t, ph, "cashier")
	te.db.Exec(`UPDATE users SET can_use_shelf = $1 WHERE id = $2`, ticked, id) //nolint:errcheck
	return ph
}

// A guard that never lets the person move (like Pocket Cashier's open till).
func holdTill(context.Context, int64, string) (bool, string) { return true, "Close the till first" }

func TestShelfLoginKeepsTheTill(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t, holdTill)
	ph := te.shelfCashier(t, true)
	till := te.signIn(t, "dev-A", "Counter PC", ph)
	r := te.shelf("dev-B", http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: ph, Pin: "1234", App: shelfApp}, nil)
	if r.Code != http.StatusOK {
		t.Fatalf("Shelf sign-in: %d %s", r.Code, r.Body)
	}
	phone := sessionCookie(t, r)
	if r := te.on("dev-A", http.MethodGet, "/api/v1/till/config", nil, till); r.Code != http.StatusOK {
		t.Fatalf("the till was thrown off: %d %s", r.Code, r.Body)
	}
	if r := te.shelf("dev-B", http.MethodGet, "/api/v1/me", nil, phone); r.Code != http.StatusOK || decode[Me](t, r).MovedTo != "" {
		t.Fatalf("Shelf /me: %d %s", r.Code, r.Body)
	}
	if r := te.shelf("dev-B", http.MethodGet, "/api/v1/admin/units", nil, phone); r.Code != http.StatusOK {
		t.Fatalf("a Shelf call from the phone: %d %s", r.Code, r.Body)
	}
	// A Shelf session sells nothing: the till stays on the PC.
	if r := te.shelf("dev-B", http.MethodGet, "/api/v1/till/config", nil, phone); r.Code != http.StatusForbidden {
		t.Fatalf("a Shelf session reached the till: %d %s", r.Code, r.Body)
	}
}

// The header alone is nothing: an admin's normal session sending it is still
// held to one device, and a cashier's normal session still can't reach admin.
func TestShelfHeaderIsNotAPass(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t)
	ph := devPhone()
	te.seedUser(t, ph, "admin")
	a := te.signIn(t, "dev-A", "Phone 1", ph)
	te.signIn(t, "dev-B", "Counter PC", ph)
	if r := te.shelf("dev-A", http.MethodGet, "/api/v1/admin/units", nil, a); r.Code != http.StatusForbidden || errCode(r) != "SESSION_LOCKED" {
		t.Fatalf("an admin's header dodged one device: %d %s", r.Code, r.Body)
	}
	cph := te.shelfCashier(t, true)
	ck := te.signIn(t, "dev-C", "Counter 2", cph)
	if r := te.shelf("dev-C", http.MethodGet, "/api/v1/admin/units", nil, ck); r.Code != http.StatusForbidden {
		t.Fatalf("a ticked cashier's till session + header reached admin: %d", r.Code)
	}
}

func TestShelfLoginNoAccess(t *testing.T) {
	te := newTestEnv(t)
	ph := te.shelfCashier(t, false)
	r := te.shelf("dev-B", http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: ph, Pin: "1234", App: shelfApp}, nil)
	if r.Code != http.StatusForbidden || errCode(r) != "SHELF_NO_ACCESS" {
		t.Fatalf("unticked: %d %s", r.Code, r.Body)
	}
	for _, c := range r.Result().Cookies() {
		if c.Name == "pos_token" && c.Value != "" {
			t.Fatal("a cookie was set for a refused Shelf sign-in")
		}
	}
	if r := te.shelf("dev-B", http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: ph, Pin: "9999", App: shelfApp}, nil); r.Code != http.StatusUnauthorized {
		t.Fatalf("wrong PIN: %d", r.Code)
	}
}

func TestShelfHeaderNeedsTheTick(t *testing.T) {
	te := newTestEnv(t)
	oneDevice(t)
	ph := te.shelfCashier(t, false)
	a := te.signIn(t, "dev-A", "Phone 1", ph)
	te.signIn(t, "dev-B", "Counter PC", ph)
	if r := te.shelf("dev-A", http.MethodGet, "/api/v1/till/config", nil, a); r.Code != http.StatusForbidden || errCode(r) != "SESSION_LOCKED" {
		t.Fatalf("the header alone let an unticked person off the rule: %d %s", r.Code, r.Body)
	}
}
