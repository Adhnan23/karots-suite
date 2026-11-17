package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Only the system user reads and saves Sell page layouts; a bad one is refused and
// nothing changes; the till's config carries them.
func TestSellLayoutSystemOnly(t *testing.T) {
	te := newTestEnv(t)
	ck := systemLogin(t, te, "0770009121")
	var before SellLayoutPage
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/sell-layout", nil, ck).Body.Bytes(), &before) //nolint:errcheck
	defer te.do(http.MethodPut, "/api/v1/system/sell-layout", SellLayoutSave{Layouts: before.Layouts}, ck)

	good := map[string]SellLayout{"1366x768": {Arr: "PY_C", Fr: []float64{.6, .4}, Split: .55, Card: "S", Text: 90}}
	rec := te.do(http.MethodPut, "/api/v1/system/sell-layout", SellLayoutSave{Layouts: good}, ck)
	if rec.Code != 200 {
		t.Fatalf("save %d %s", rec.Code, rec.Body)
	}
	var page SellLayoutPage
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/sell-layout", nil, ck).Body.Bytes(), &page) //nolint:errcheck
	if l := page.Layouts["1366x768"]; l.Arr != "PY_C" || l.Card != "S" || l.Text != 90 || page.Screens == nil {
		t.Fatalf("got %+v", page)
	}

	narrow := map[string]SellLayout{"1366x768": {Arr: "PCY", Fr: []float64{.45, .3, .25}}}
	if rec := te.do(http.MethodPut, "/api/v1/system/sell-layout", SellLayoutSave{Layouts: narrow}, ck); rec.Code != 400 {
		t.Fatalf("too narrow accepted: %d", rec.Code)
	}
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/system/sell-layout", nil, ck).Body.Bytes(), &page) //nolint:errcheck
	if page.Layouts["1366x768"].Arr != "PY_C" {
		t.Fatal("a refused save changed the layout")
	}

	te.seedUser(t, "0770009122", "admin")
	admin := login(t, te, "0770009122")
	if rec := te.do(http.MethodPut, "/api/v1/system/sell-layout", SellLayoutSave{Layouts: good}, admin); rec.Code < 400 {
		t.Fatalf("an admin saved a layout: %d", rec.Code)
	}

	var cfg TillConfig
	json.Unmarshal(te.do(http.MethodGet, "/api/v1/till/config", nil, admin).Body.Bytes(), &cfg) //nolint:errcheck
	if cfg.SellLayouts["1366x768"].Arr != "PY_C" {
		t.Fatalf("the till didn't get the layout: %+v", cfg.SellLayouts)
	}
}
