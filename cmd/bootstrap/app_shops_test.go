package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type shopsApp struct {
	h    http.Handler
	path string
}

func newShopsApp(t *testing.T, b func(buildOpts, io.Writer) (buildResult, error)) shopsApp {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shops.json")
	all := []manifest{{Key: "alpha", Name: "Alpha"}}
	t.Setenv("SUITE_RELEASE_KEY", "")
	return shopsApp{appHandler(all, b, shopsCfg{Path: path, Out: filepath.Join(dir, "dist"), Setup: fakeSetup, Env: filepath.Join(dir, ".env")}), path}
}

func (a shopsApp) do(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	method := "POST"
	if body == "" {
		method = "GET"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	a.h.ServeHTTP(rec, req)
	return rec
}

func (a shopsApp) stored(t *testing.T) Shop {
	t.Helper()
	s, err := LoadShops(a.path)
	if err != nil || len(s.Shops) != 1 {
		t.Fatalf("stored: %+v %v", s, err)
	}
	return s.Shops[0]
}

const newShopBody = `{"key":"al-raj","name":"Al Raj","os":"linux","arch":"amd64","mode":"server"}`

func TestAppShopsCreateListMasked(t *testing.T) {
	a := newShopsApp(t, nil)
	if rec := a.do(t, "/api/shops", newShopBody); rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(t, "/api/shops", newShopBody); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate %d", rec.Code)
	}
	if rec := a.do(t, "/api/shops", `{"key":"Bad Key","name":"x","os":"linux","arch":"amd64","mode":"file"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad key %d", rec.Code)
	}
	s := a.stored(t)
	body := a.do(t, "/api/shops", "").Body.String()
	for _, secret := range []string{s.JWTSecret, s.DB.Password, s.DB.SuperPassword} {
		if strings.Contains(body, secret) {
			t.Fatalf("list leaks a secret: %s", body)
		}
	}
	if !strings.Contains(body, `"installId":"`+s.InstallID+`"`) {
		t.Fatalf("list %s", body)
	}
}

// The edit form sends the masked shop back: masked secrets stay as they were.
func TestAppShopsUpdateKeepsSecrets(t *testing.T) {
	a := newShopsApp(t, nil)
	a.do(t, "/api/shops", newShopBody)
	before := a.stored(t)
	var list struct{ Shops []Shop }
	json.Unmarshal(a.do(t, "/api/shops", "").Body.Bytes(), &list)
	m := list.Shops[0]
	m.Port, m.Name, m.InstallID = 3005, "Al Raj Stores", "HACKED"
	m.Env = map[string]string{"JWT_EXPIRES_IN": "10h"}
	raw, _ := json.Marshal(m)
	if rec := a.do(t, "/api/shops/al-raj", string(raw)); rec.Code != 200 {
		t.Fatalf("update %d %s", rec.Code, rec.Body)
	}
	after := a.stored(t)
	if after.Port != 3005 || after.Name != "Al Raj Stores" || after.Env["JWT_EXPIRES_IN"] != "10h" {
		t.Fatalf("not updated: %+v", after)
	}
	if after.JWTSecret != before.JWTSecret || after.DB.Password != before.DB.Password || after.InstallID != before.InstallID {
		t.Fatalf("secrets / install id changed: %+v", after)
	}
	m.Mode = "file"
	raw, _ = json.Marshal(m)
	if rec := a.do(t, "/api/shops/al-raj", string(raw)); rec.Code != 200 {
		t.Fatalf("to file %d %s", rec.Code, rec.Body)
	}
	if f := a.stored(t); f.DB != nil || f.JWTSecret != "" || f.FileDBPort != 5434 {
		t.Fatalf("file mode: %+v", f)
	}
}

func TestAppShopsRotateRevealEnvDelete(t *testing.T) {
	a := newShopsApp(t, nil)
	a.do(t, "/api/shops", newShopBody)
	before := a.stored(t)
	if rec := a.do(t, "/api/shops/al-raj/rotate", `{"what":"jwt"}`); rec.Code != 200 || a.stored(t).JWTSecret == before.JWTSecret {
		t.Fatalf("rotate %d", rec.Code)
	}
	if rec := a.do(t, "/api/shops/al-raj/reveal", `{}`); !strings.Contains(rec.Body.String(), a.stored(t).JWTSecret) {
		t.Fatalf("reveal %s", rec.Body)
	}
	rec := a.do(t, "/api/shops/al-raj/env", `{}`)
	if !strings.Contains(rec.Body.String(), "DATABASE_URL=postgres://") || !strings.Contains(rec.Header().Get("Content-Disposition"), `al-raj.env`) {
		t.Fatalf("env %s %v", rec.Body, rec.Header())
	}
	if rec := a.do(t, "/api/shops/al-raj/delete", `{"confirm":"nope"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete without confirm %d", rec.Code)
	}
	if rec := a.do(t, "/api/shops/al-raj/delete", `{"confirm":"al-raj"}`); rec.Code != 200 {
		t.Fatalf("delete %d", rec.Code)
	}
	if s, _ := LoadShops(a.path); len(s.Shops) != 0 {
		t.Fatal("not deleted")
	}
	if rec := a.do(t, "/api/shops/ghost/rotate", `{"what":"jwt"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown shop %d", rec.Code)
	}
}

func TestAppShopsBuild(t *testing.T) {
	var got buildOpts
	a := newShopsApp(t, fakeBuild(&got))
	a.do(t, "/api/shops", newShopBody)
	body := a.do(t, "/api/shops/al-raj/build", `{"installer":true}`).Body.String()
	if !strings.Contains(body, resultMark) || got.InstallID != a.stored(t).InstallID {
		t.Fatalf("build: %s", body)
	}
	if a.stored(t).LastBuilt == nil {
		t.Fatal("lastBuilt not saved")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.path), "dist", "al-raj", "install.conf")); err != nil {
		t.Fatal(err)
	}
	failing := newShopsApp(t, func(buildOpts, io.Writer) (buildResult, error) { return buildResult{}, fmt.Errorf("boom") })
	failing.do(t, "/api/shops", newShopBody)
	if b := failing.do(t, "/api/shops/al-raj/build", `{"installer":false}`).Body.String(); !strings.Contains(b, errorMark+"boom") {
		t.Fatalf("failure: %s", b)
	}
}

func TestAppShopsRefuseTextPlain(t *testing.T) {
	a := newShopsApp(t, nil)
	req := httptest.NewRequest("POST", "/api/shops", strings.NewReader(newShopBody))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("%d", rec.Code)
	}
}

// Build all: every shop in turn; one failing doesn't stop the rest, and the end
// line names the failures.
func TestAppShopsBuildAll(t *testing.T) {
	var got buildOpts
	ok := fakeBuild(&got)
	a := newShopsApp(t, func(o buildOpts, log io.Writer) (buildResult, error) {
		if strings.HasSuffix(o.Out, "broken") {
			return buildResult{}, fmt.Errorf("boom")
		}
		return ok(o, log)
	})
	a.do(t, "/api/shops", newShopBody)
	a.do(t, "/api/shops", `{"key":"broken","name":"Broken","os":"linux","arch":"amd64","mode":"file"}`)
	a.do(t, "/api/shops", `{"key":"zed","name":"Zed","os":"windows","arch":"amd64","mode":"file"}`)
	body := a.do(t, "/api/shops/build-all", `{"installer":true}`).Body.String()
	for _, want := range []string{"=== Al Raj", "=== Broken", "=== Zed", "✗ broken: boom", errorMark + "1 of 3 shops failed: broken"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	shops, _ := LoadShops(a.path)
	for _, s := range shops.Shops {
		if (s.LastBuilt != nil) == (s.Key == "broken") {
			t.Errorf("%s lastBuilt %v", s.Key, s.LastBuilt)
		}
	}
}
