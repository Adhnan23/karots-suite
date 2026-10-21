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

// systemLogin signs in a new admin, then flags it system (log in BEFORE flagging:
// system users' PINs go through the rotating validator).
func systemLogin(t *testing.T, te *testEnv, phone string) *http.Cookie {
	t.Helper()
	id := te.seedUser(t, phone, "admin")
	ck := login(t, te, phone)
	te.db.Exec(`UPDATE users SET is_system=true WHERE id=$1`, id) //nolint:errcheck
	return ck
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

func TestAppearanceReceiptStyleAndLegacyLook(t *testing.T) {
	te := newTestEnv(t)
	ck := systemLogin(t, te, "0770009013")
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() {
		te.do(http.MethodPut, "/api/v1/system/appearance", before, ck)
		te.db.Exec(`UPDATE settings SET skin_custom=$1, skin_radius=$2`, before.BrandColor, before.Radius) //nolint:errcheck
	})
	te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "studio", ReceiptStyle: "classic"}, ck)
	te.db.Exec(`UPDATE settings SET skin_custom='#0f766e', skin_radius='round'`) //nolint:errcheck

	// A receipt style change on the same palette keeps a legacy brand + radius.
	rec := te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "studio", ReceiptStyle: "ticket", BrandColor: "#ff0000"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if rec.Code != 200 || got.BrandColor != "#0f766e" || got.Radius != "round" || got.ReceiptStyle != "ticket" {
		t.Fatalf("%d %+v", rec.Code, got)
	}
	var cfg TillConfig
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/till/config", nil, ck).Body.Bytes(), &cfg) //nolint:errcheck
	if cfg.ReceiptStyle != "ticket" {
		t.Fatalf("till config receiptStyle = %q", cfg.ReceiptStyle)
	}
	// Choosing a palette clears them; a junk style reads as classic.
	rec = te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "night", ReceiptStyle: "modern"}, ck)
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.BrandColor != "" || got.Radius != "rounded" || got.ReceiptStyle != "classic" || got.Palette != "night" {
		t.Fatalf("palette change kept the legacy look: %+v", got)
	}
}

func TestAppearancePaletteRoundTripAndFallback(t *testing.T) {
	te := newTestEnv(t)
	ck := systemLogin(t, te, "0770009014")
	var before Appearance
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/appearance", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	t.Cleanup(func() { te.do(http.MethodPut, "/api/v1/system/appearance", before, ck) })

	rec := te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "lagoon"}, ck)
	var got Appearance
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if rec.Code != 200 || got.Palette != "lagoon" {
		t.Fatalf("palette round-trip: %d %+v", rec.Code, got)
	}

	rec = te.do(http.MethodPut, "/api/v1/system/appearance", Appearance{Palette: "nope"}, ck)
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	if got.Palette != "studio" {
		t.Fatalf("unknown palette should fall back to studio: %+v", got)
	}
}
