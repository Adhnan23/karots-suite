package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminUsers(t *testing.T) {
	te := newTestEnv(t)
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }
	if rec := te.do(http.MethodGet, "/api/v1/admin/users", nil, te.login(t, ph(1), "manager")); rec.Code != http.StatusForbidden {
		t.Fatalf("manager on users: %d", rec.Code)
	}
	adminPhone := ph(2)
	ck := te.login(t, adminPhone, "admin")
	for _, u := range decode[[]StaffUser](t, te.do(http.MethodGet, "/api/v1/admin/users", nil, ck)) {
		if u.Phone == "0000000001" {
			t.Fatal("system user listed")
		}
	}
	phone := ph(3)
	rec := te.do(http.MethodPost, "/api/v1/admin/users", StaffRequest{Name: "T Nimali", Phone: phone, Role: "cashier", Pin: "4321", CanSeeCost: true}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	u := decode[StaffUser](t, rec)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM users WHERE id=$1`, u.ID) }) //nolint:errcheck
	if u.Role != "cashier" || !u.CanSeeCost || u.CanManageCredit {
		t.Fatalf("created: %+v", u)
	}
	login := func(pin string) int {
		return te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: pin}, nil).Code
	}
	if c := login("4321"); c != http.StatusOK {
		t.Fatalf("new user login: %d", c)
	}
	// Promote to manager with credit rights and a new PIN.
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), StaffRequest{Name: "T Nimali", Phone: phone, Role: "manager", Pin: "9876", CanManageCredit: true}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if login("4321") == http.StatusOK || login("9876") != http.StatusOK {
		t.Fatal("PIN reset did not take")
	}
	var me int64
	te.db.Get(&me, `SELECT id FROM users WHERE phone=$1`, adminPhone) //nolint:errcheck
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", me), nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("deactivate self: %d", rec.Code)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", u.ID), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate: %d", rec.Code)
	}
	if login("9876") == http.StatusOK {
		t.Fatal("deactivated user can still sign in")
	}
	if rec := te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%d/activate", u.ID), nil, ck); rec.Code != http.StatusNoContent || login("9876") != http.StatusOK {
		t.Fatalf("reactivate: %d", rec.Code)
	}
}
