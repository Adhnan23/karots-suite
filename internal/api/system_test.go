package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func login(t *testing.T, te *testEnv, phone string) *http.Cookie {
	t.Helper()
	return sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "1234"}, nil))
}

func TestKioskExitHiddenFromCashier(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009011", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/kiosk/exit", nil, login(t, te, "0770009011")); rec.Code != 404 {
		t.Fatalf("cashier kiosk exit: %d", rec.Code)
	}
}

func TestAppearanceSystemOnly(t *testing.T) {
	te := newTestEnv(t)
	te.seedUser(t, "0770009012", "admin")
	if rec := te.do(http.MethodGet, "/api/v1/system/appearance", nil, login(t, te, "0770009012")); rec.Code != 404 {
		t.Fatalf("admin saw system appearance: %d", rec.Code)
	}
}

func TestAppearanceRoundTripAndSanitises(t *testing.T) {
	te := newTestEnv(t)
	id := te.seedUser(t, "0770009013", "admin")
	ck := login(t, te, "0770009013") // log in BEFORE flagging: system users' PINs go through the rotating validator
	te.db.Exec(`UPDATE users SET is_system=true WHERE id=$1`, id) //nolint:errcheck
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() { te.do(http.MethodPut, "/api/v1/system/appearance", before, ck) })

	rec := te.do(http.MethodPut, "/api/v1/system/appearance",
		Appearance{BrandColor: "#0f766e", Radius: "round", Density: "compact", ReceiptStyle: "boxed"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if rec.Code != 200 || got.BrandColor != "#0f766e" || got.Radius != "round" || got.Density != "compact" || got.ReceiptStyle != "boxed" {
		t.Fatalf("%d %+v", rec.Code, got)
	}
	rec = te.do(http.MethodPut, "/api/v1/system/appearance",
		Appearance{BrandColor: "red;}", Radius: "blob", Density: "x", ReceiptStyle: "y"}, ck)
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.BrandColor != "" || got.Radius != "rounded" || got.Density != "comfortable" || got.ReceiptStyle != "classic" {
		t.Fatalf("junk not sanitised: %+v", got)
	}
}

func TestAppearancePaletteRoundTripAndFallback(t *testing.T) {
	te := newTestEnv(t)
	id := te.seedUser(t, "0770009014", "admin")
	ck := login(t, te, "0770009014")
	te.db.Exec(`UPDATE users SET is_system=true WHERE id=$1`, id) //nolint:errcheck
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() { te.do(http.MethodPut, "/api/v1/system/appearance", before, ck) })

	rec := te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "night", BrandColor: "#6e5bff"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if rec.Code != 200 || got.Palette != "night" || got.BrandColor != "#6e5bff" {
		t.Fatalf("palette round-trip: %d %+v", rec.Code, got)
	}

	rec = te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "bazaar"}, ck)
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.Palette != "studio" {
		t.Fatalf("unknown palette should fall back to studio: %+v", got)
	}
}
