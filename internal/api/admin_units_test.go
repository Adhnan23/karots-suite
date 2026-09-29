package api

import (
	"net/http"
	"strconv"
	"testing"
)

func TestAdminUnits(t *testing.T) {
	te := newTestEnv(t)
	cashier := te.login(t, "0770009501", "cashier")
	if rec := te.do(http.MethodGet, "/api/v1/admin/units", nil, cashier); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier reached admin: %d", rec.Code)
	}
	ck := te.login(t, "0770009502", "manager")
	te.db.Exec(`DELETE FROM units WHERE abbreviation='tbx'`) //nolint:errcheck
	rec := te.do(http.MethodPost, "/api/v1/admin/units", UnitRequest{Name: " Test box ", Abbreviation: "tbx"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	u := decode[Unit](t, rec)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM units WHERE id=$1`, u.ID) }) //nolint:errcheck
	if u.Name != "Test box" {
		t.Fatalf("name not trimmed: %q", u.Name)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/units", UnitRequest{Name: "Other", Abbreviation: "tbx"}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/units", UnitRequest{Name: "", Abbreviation: "x"}, ck); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("empty name: %d", rec.Code)
	}
	path := "/api/v1/admin/units/" + strconv.FormatInt(u.ID, 10)
	if rec := te.do(http.MethodPut, path, UnitRequest{Name: "Test boxes", Abbreviation: "tbx", AllowDecimal: true}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodDelete, path, nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	te.seedTill(t) // products on unit 1: it's in use and can't go
	if rec := te.do(http.MethodDelete, "/api/v1/admin/units/1", nil, ck); rec.Code != http.StatusConflict {
		t.Fatalf("in-use delete: %d", rec.Code)
	}
}
