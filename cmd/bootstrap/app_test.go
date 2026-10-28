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

func testApp(build func(buildOpts, io.Writer) (buildResult, error)) http.Handler {
	return appHandler([]manifest{{Key: "alpha", Name: "Alpha", Description: "Does alpha things."}, {Key: "beta", Name: "Beta"}}, build, shopsCfg{Path: filepath.Join(os.TempDir(), "bootstrap-test-no-shops.json"), Env: filepath.Join(os.TempDir(), "bootstrap-test.env")})
}

func TestAppState(t *testing.T) {
	rec := httptest.NewRecorder()
	testApp(nil).ServeHTTP(rec, httptest.NewRequest("GET", "/api/state", nil))
	var st appState
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || len(st.Plugins) != 2 || st.Plugins[0].Name != "Alpha" || st.Name != "karots-suite" {
		t.Fatalf("state %d %s (%v)", rec.Code, rec.Body, err)
	}
	if rec := get(t, testApp(nil), "/"); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>") {
		t.Fatalf("page: %d", rec.Code)
	}
}

// Bad input is refused as JSON before any build starts.
func TestAppBuildRefusesBadInput(t *testing.T) {
	ran := false
	app := testApp(func(buildOpts, io.Writer) (buildResult, error) { ran = true; return buildResult{}, nil })
	for _, body := range []string{
		`{"plugins":["gamma"],"os":"linux","arch":"amd64","name":"shop"}`,
		`{"plugins":[],"os":"plan9","arch":"amd64","name":"shop"}`,
		`{"plugins":[],"os":"linux","arch":"amd64","name":"shop","installId":"not an id!"}`,
		`{"plugins":[],"os":"linux","arch":"amd64","name":"../shop"}`,
	} {
		rec := post(t, app, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s → %d %s, want 400 JSON", body, rec.Code, rec.Body)
		}
	}
	if ran {
		t.Fatal("a build ran on bad input")
	}
}

// A good request streams the log and ends with the result line; a custom
// install id reaches the build normalised.
func TestAppBuildStreamsResult(t *testing.T) {
	var got buildOpts
	app := testApp(func(o buildOpts, log io.Writer) (buildResult, error) {
		got = o
		fmt.Fprintln(log, "→ go build")
		return buildResult{Binary: "dist/shop", InstallID: o.InstallID, SupportPIN: "123456"}, nil
	})
	rec := post(t, app, `{"plugins":["beta"],"os":"windows","arch":"amd64","name":"shop","installId":" 3fa2c91b "}`)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, "→ go build") || !strings.Contains(body, resultMark+`{"binary":"dist/shop","installId":"3FA2C91B","supportPin":"123456"}`) {
		t.Fatalf("%d %s", rec.Code, body)
	}
	if len(got.Plugins) != 1 || got.Plugins[0].Key != "beta" || got.OS != "windows" || got.Out != "dist" {
		t.Fatalf("opts %+v", got)
	}
	failing := testApp(func(buildOpts, io.Writer) (buildResult, error) { return buildResult{}, fmt.Errorf("make web: exit 1") })
	if b := post(t, failing, `{"plugins":[],"os":"linux","arch":"amd64","name":"shop"}`).Body.String(); !strings.Contains(b, errorMark+"make web: exit 1") {
		t.Fatalf("failure line missing: %s", b)
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/build", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec
}

// Another site open in the browser can't start a build: a cross-site form or
// text/plain POST is refused, only a same-origin JSON call (which needs a CORS
// preflight this server never answers) gets through.
func TestAppBuildRefusesCrossSitePosts(t *testing.T) {
	ran := false
	app := testApp(func(buildOpts, io.Writer) (buildResult, error) { ran = true; return buildResult{}, nil })
	req := httptest.NewRequest("POST", "/api/build", strings.NewReader(`{"plugins":[],"os":"linux","arch":"amd64","name":"shop"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType || ran {
		t.Fatalf("text/plain POST: %d, ran=%v", rec.Code, ran)
	}
}

func TestAppDemoBuildStreamsAndPassesFresh(t *testing.T) {
	var fresh bool
	demoBuild = func(_ []manifest, f bool, log io.Writer) (buildResult, error) {
		fresh = f
		fmt.Fprintln(log, "building")
		return buildResult{Folder: "dist/demo", InstallID: "DEMO1234", Unlock: "123456"}, nil
	}
	t.Cleanup(func() { demoBuild = buildDemo })
	req := httptest.NewRequest("POST", "/api/demo", strings.NewReader(`{"fresh":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testApp(nil).ServeHTTP(rec, req)
	if !fresh || !strings.Contains(rec.Body.String(), "building") || !strings.Contains(rec.Body.String(), `@@RESULT {"binary":"","folder":"dist/demo","installId":"DEMO1234","supportPin":"","unlock":"123456"}`) {
		t.Fatalf("fresh %v, body %s", fresh, rec.Body)
	}
}
