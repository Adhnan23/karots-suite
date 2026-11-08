package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// The Users page's "Can use Karots Shelf" tick, and /me's role-aware answer.
func TestUserShelfFlag(t *testing.T) {
	te := newTestEnv(t)
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }
	ck := te.login(t, ph(11), "admin")
	phone := ph(12)
	rec := te.do(http.MethodPost, "/api/v1/admin/users", StaffRequest{Name: "T Shelf", Phone: phone, Role: "cashier", Pin: "4321", CanUseShelf: true}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	u := decode[StaffUser](t, rec)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM users WHERE id=$1`, u.ID) }) //nolint:errcheck
	if !u.CanUseShelf {
		t.Fatalf("created without the tick: %+v", u)
	}
	me := func(c *http.Cookie) bool {
		return decode[Me](t, te.do(http.MethodGet, "/api/v1/me", nil, c)).Flags.MayUseShelf
	}
	cashier := te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "4321"}, nil).Result().Cookies()[0]
	if !me(cashier) {
		t.Fatal("ticked cashier: /me says no Shelf")
	}
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), StaffRequest{Name: "T Shelf", Phone: phone, Role: "cashier"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("untick: %d %s", rec.Code, rec.Body)
	}
	for _, x := range decode[[]StaffUser](t, te.do(http.MethodGet, "/api/v1/admin/users", nil, ck)) {
		if x.ID == u.ID && x.CanUseShelf {
			t.Fatal("still ticked after the update")
		}
	}
	if me(cashier) {
		t.Fatal("unticked cashier: /me still says Shelf")
	}
	if !me(ck) {
		t.Fatal("admin: /me says no Shelf")
	}
}
