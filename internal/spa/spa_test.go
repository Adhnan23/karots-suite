package spa

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v4"
)

func serve(t *testing.T, fsys fstest.MapFS, path string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.GET("/*", Handler(fsys))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

var site = fstest.MapFS{
	"index.html":        {Data: []byte("<html>app</html>")},
	"assets/app-abc.js": {Data: []byte("js")},
}

func TestDeepLinkServesIndexNoCache(t *testing.T) {
	rec := serve(t, site, "/admin/reports/sales?from=2026-01-01")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("%d %q %q", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
	}
}

func TestHashedAssetImmutable(t *testing.T) {
	rec := serve(t, site, "/assets/app-abc.js")
	if rec.Code != 200 || rec.Body.String() != "js" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestMissingAssetIs404NotIndex(t *testing.T) {
	if rec := serve(t, site, "/assets/gone.js"); rec.Code != 404 {
		t.Fatalf("%d", rec.Code)
	}
}

func TestNotBuiltGives503(t *testing.T) {
	if rec := serve(t, fstest.MapFS{}, "/"); rec.Code != 503 {
		t.Fatalf("%d", rec.Code)
	}
}

// The browser only offers "Install" when the manifest comes back as JSON.
func TestManifestServedAsManifestJSON(t *testing.T) {
	rec := serve(t, fstest.MapFS{"index.html": {Data: []byte("<html>")}, "manifest.webmanifest": {Data: []byte("{}")}}, "/manifest.webmanifest")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
