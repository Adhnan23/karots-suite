package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karots-suite/internal/shopconf"
)

// fakePG serves body as the PostgreSQL installer, pinned to sum, and counts downloads.
func fakePG(t *testing.T, body, sum string) *int {
	t.Helper()
	hits := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { *hits++; io.WriteString(w, body) }))
	t.Cleanup(srv.Close)
	oldURL, oldSum, oldCache := pgURL, pgSHA256, pgCache
	t.Cleanup(func() { pgURL, pgSHA256, pgCache = oldURL, oldSum, oldCache })
	pgURL, pgSHA256 = srv.URL, sum
	dir := t.TempDir()
	pgCache = func() (string, error) { return dir, nil }
	return hits
}

func upperSum(s string) string {
	h := sha256.Sum256([]byte(s))
	return strings.ToUpper(hex.EncodeToString(h[:]))
}

func TestPGInstallerCachedOnce(t *testing.T) {
	hits := fakePG(t, "EXE", upperSum("EXE"))
	p1, err := pgInstaller(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := pgInstaller(io.Discard)
	if err != nil || p1 != p2 || *hits != 1 || filepath.Base(p1) != shopconf.PGInstallerName {
		t.Fatalf("p1 %s p2 %s hits %d err %v", p1, p2, *hits, err)
	}
}

func TestPGInstallerBadDownload(t *testing.T) {
	fakePG(t, "TAMPERED", upperSum("EXE"))
	p, err := pgInstaller(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err %v", err)
	}
	if _, serr := os.Stat(p); p != "" && serr == nil {
		t.Fatal("a damaged download must not be kept")
	}
}

// Only a Windows main PC that installs its own PostgreSQL gets the 4th file.
func TestBuildShopAddsPGInstaller(t *testing.T) {
	src := filepath.Join(t.TempDir(), shopconf.PGInstallerName)
	os.WriteFile(src, []byte("EXE"), 0o644)
	for _, c := range []struct {
		os, mode string
		want     bool
	}{{"windows", "server", true}, {"linux", "server", false}, {"windows", "file", false}} {
		out := t.TempDir()
		s := mustShop(t, "al-raj", c.os, c.mode)
		var got buildOpts
		if _, err := buildShop(&s, nil, shopBuildOpts{Out: out, Installer: true, PGInstaller: src}, fakeBuild(&got), fakeSetup, io.Discard); err != nil {
			t.Fatal(err)
		}
		_, err := os.Stat(filepath.Join(out, "al-raj", shopconf.PGInstallerName))
		if (err == nil) != c.want {
			t.Errorf("%s/%s: installer copied = %v, want %v", c.os, c.mode, err == nil, c.want)
		}
	}
}
