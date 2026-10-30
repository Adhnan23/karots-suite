package pocketcashier

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSWListsAssets(t *testing.T) {
	en := newEnv(t)
	rec := en.do(http.MethodGet, "/sw.js", nil, nil, "")
	b := rec.Body.String()
	if rec.Code != http.StatusOK || rec.Header().Get("Service-Worker-Allowed") != "/" || !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	for _, want := range []string{`"/index.html"`, `"/assets/`, `"test"`, `"/x/pocketcashier/"`} {
		if !strings.Contains(b, want) {
			t.Fatalf("sw.js lacks %s", want)
		}
	}
}

func TestManifest(t *testing.T) {
	en := newEnv(t)
	rec := en.do(http.MethodGet, "/manifest.webmanifest", nil, nil, "")
	var m struct {
		StartURL string `json:"start_url"`
		Scope    string `json:"scope"`
		Display  string `json:"display"`
		Icons    []struct{ Src string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || m.StartURL != "/x/pocketcashier/" || m.Scope != "/" || m.Display != "standalone" || len(m.Icons) != 3 {
		t.Fatalf("%+v %v", m, err)
	}
}
