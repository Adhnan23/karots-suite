package api

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"testing"
	"time"
)

func TestAdminSettings(t *testing.T) {
	te := newTestEnv(t)
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }
	if rec := te.do(http.MethodGet, "/api/v1/admin/settings", nil, te.login(t, ph(1), "cashier")); rec.Code != http.StatusForbidden {
		t.Fatalf("cashier: %d", rec.Code)
	}
	ck := te.login(t, ph(2), "manager")
	page := decode[SettingsPage](t, te.do(http.MethodGet, "/api/v1/admin/settings", nil, ck))
	orig := page.Settings
	var logo string
	te.db.Get(&logo, `SELECT logo_data FROM settings WHERE id=1`) //nolint:errcheck
	t.Cleanup(func() {
		te.do(http.MethodPut, "/api/v1/admin/settings", orig, ck)
		te.db.Exec(`UPDATE settings SET logo_data=$1 WHERE id=1`, logo) //nolint:errcheck
	})

	in := orig
	in.ShopName, in.ReceiptFooter, in.StockTakeEnabled, in.TaxRegistered = "Test Shop", "Come again", !orig.StockTakeEnabled, true
	rec := te.do(http.MethodPut, "/api/v1/admin/settings", in, ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	got := decode[SettingsPage](t, rec).Settings
	if got.ShopName != "Test Shop" || got.ReceiptFooter != "Come again" || got.StockTakeEnabled == orig.StockTakeEnabled {
		t.Fatalf("saved: %+v", got)
	}
	me := decode[Me](t, te.do(http.MethodGet, "/api/v1/me", nil, ck))
	if me.StockTakeEnabled != in.StockTakeEnabled || !me.TaxRegistered || me.ShopName != "Test Shop" {
		t.Fatalf("/me after save: %+v", me)
	}

	for name, bad := range map[string]func(*ShopSettings){
		"receipt width": func(s *ShopSettings) { s.ReceiptWidth = "100" },
		"no shop name":  func(s *ShopSettings) { s.ShopName = " " },
		"sale type":     func(s *ShopSettings) { s.DefaultSaleType = "bulk" },
		"locker":        func(s *ShopSettings) { s.DefaultLockerID = 999999999 },
	} {
		b := orig
		bad(&b)
		if rec := te.do(http.MethodPut, "/api/v1/admin/settings", b, ck); rec.Code < 400 || rec.Code >= 500 {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}

	if rec := te.upload("/api/v1/admin/settings/logo", "logo", "x.png", []byte("not an image"), ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad logo: %d", rec.Code)
	}
	var img bytes.Buffer
	m := image.NewRGBA(image.Rect(0, 0, 900, 300))
	m.Set(10, 10, color.Black)
	png.Encode(&img, m) //nolint:errcheck
	rec = te.upload("/api/v1/admin/settings/logo", "logo", "logo.png", img.Bytes(), ck)
	if p := decode[SettingsPage](t, rec); rec.Code != http.StatusOK || !p.LogoUploaded || p.Logo == "" {
		t.Fatalf("logo: %d %+v", rec.Code, p.LogoUploaded)
	}
	if p := decode[SettingsPage](t, te.do(http.MethodPost, "/api/v1/admin/settings/logo/clear", nil, ck)); p.LogoUploaded {
		t.Fatal("logo not cleared")
	}

	b := orig
	b.ReceiptPrinter = ""
	te.do(http.MethodPut, "/api/v1/admin/settings", b, ck)
	if rec := te.do(http.MethodPost, "/api/v1/admin/settings/printer/test", nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("test print with no printer: %d", rec.Code)
	}
}
