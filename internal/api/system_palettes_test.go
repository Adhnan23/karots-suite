package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestPalettesSystemOnly(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009015", "admin")
	rec := te.do(http.MethodPost, "/api/v1/system/palettes", PaletteInput{Name: "TEST x", Base: "studio", CSS: "a"}, login(t, te, "0770009015"))
	if rec.Code != 404 {
		t.Fatalf("admin made a palette: %d", rec.Code)
	}
}

func TestPaletteCRUDAndActiveCSS(t *testing.T) {
	te := newTestEnv(t)
	ck := systemLogin(t, te, "0770009016")
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() {
		te.do(http.MethodPut, "/api/v1/system/appearance", before, ck)
		te.db.Exec(`DELETE FROM palettes WHERE name LIKE 'TEST %'`) //nolint:errcheck
	})

	rec := te.do(http.MethodPost, "/api/v1/system/palettes", PaletteInput{Name: "TEST Sea", Base: "lagoon", CSS: ":root{--brand:#123456}"}, ck)
	var p Palette
	json.Unmarshal(rec.Body.Bytes(), &p) //nolint:errcheck
	if rec.Code != 200 || p.ID == 0 || p.Theme != "dock" || p.CSS == "" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var list []Palette
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/palettes", nil, ck).Body.Bytes(), &list) //nolint:errcheck
	found := false
	for _, l := range list {
		found = found || (l.ID == p.ID && l.CSS == "")
	}
	if !found {
		t.Fatalf("list (without css): %+v", list)
	}
	if rec := te.do(http.MethodPost, "/api/v1/system/palettes", PaletteInput{Name: "test sea", Base: "studio", CSS: "a"}, ck); rec.Code != 409 {
		t.Fatalf("duplicate: %d", rec.Code)
	}

	skin := fmt.Sprintf("custom-%d", p.ID)
	if rec := te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "custom-999999999"}, ck); rec.Code != 404 {
		t.Fatalf("missing custom palette accepted: %d", rec.Code)
	}
	rec = te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: skin, ReceiptStyle: "classic"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.Palette != skin {
		t.Fatalf("custom palette not kept: %d %+v", rec.Code, got)
	}

	// Signed out, the login page loads the active palette.
	css := te.do(http.MethodGet, "/api/v1/appearance/palette.css", nil, nil)
	if css.Code != 200 || css.Header().Get("Content-Type") != "text/css; charset=utf-8" || css.Body.String() != ":root{--brand:#123456}" {
		t.Fatalf("palette.css: %d %q %q", css.Code, css.Header().Get("Content-Type"), css.Body)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/system/palettes/%d", p.ID), nil, ck); rec.Code != 409 {
		t.Fatalf("deleted the active palette: %d", rec.Code)
	}
	te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "studio"}, ck)
	if css := te.do(http.MethodGet, "/api/v1/appearance/palette.css", nil, nil); css.Body.Len() != 0 {
		t.Fatalf("built-in palette served css: %q", css.Body)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/system/palettes/%d", p.ID), nil, ck); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}
