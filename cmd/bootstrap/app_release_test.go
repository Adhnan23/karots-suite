package main

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// releaseApp is the shops app with a fake bucket, a temp .env and a chosen
// tree state; openStore can be swapped to fail like a missing R2 setup.
func releaseApp(t *testing.T, dirty bool) (shopsApp, *fakeStore, *error) {
	t.Helper()
	st, base := newFakeStore(t)
	t.Setenv("UPDATE_BASE_URL", base)
	t.Setenv("SUITE_RELEASE_KEY", "")
	var got buildOpts
	dir := t.TempDir()
	path := filepath.Join(dir, "shops.json")
	storeErr := new(error)
	cfg := shopsCfg{Path: path, Out: filepath.Join(dir, "dist"), Setup: fakeSetup, Env: filepath.Join(dir, ".env"),
		Store: func(io.Writer) (store, error) {
			if *storeErr != nil {
				return nil, *storeErr
			}
			return st, nil
		},
		Dirty: func() (bool, error) { return dirty, nil },
	}
	a := shopsApp{appHandler([]manifest{{Key: "alpha", Name: "Alpha"}}, fakeBuild(&got), cfg), path}
	a.do(t, "/api/shops", newShopBody)
	return a, st, storeErr
}

func TestAppRelease(t *testing.T) {
	a, st, _ := releaseApp(t, false)
	body := a.do(t, "/api/shops/al-raj/release", `{"installer":true,"notes":"faster till"}`).Body.String()
	s := a.stored(t)
	if !strings.Contains(body, resultMark) || s.Published == nil || s.LastBuilt.Kind != "release" || s.Published.Version != s.LastBuilt.Version {
		t.Fatalf("release: %s\n%+v %+v", body, s.LastBuilt, s.Published)
	}
	if _, ok := st.files[s.Channel+"/manifest.json"]; !ok {
		t.Fatal("manifest not uploaded")
	}
}

func TestAppReleaseNotSetUpThenPublish(t *testing.T) {
	a, st, storeErr := releaseApp(t, false)
	*storeErr = errR2NotSetUp
	body := a.do(t, "/api/shops/al-raj/release", `{"installer":true,"notes":"n"}`).Body.String()
	s := a.stored(t)
	if !strings.Contains(body, errorMark) || !strings.Contains(body, "R2 not set up") || s.LastBuilt == nil || s.LastBuilt.Kind != "release" || s.Published != nil {
		t.Fatalf("not set up: %s\n%+v %+v", body, s.LastBuilt, s.Published)
	}
	*storeErr = nil
	body = a.do(t, "/api/shops/al-raj/publish", `{}`).Body.String()
	s = a.stored(t)
	if !strings.Contains(body, resultMark) || s.Published == nil || s.Published.Version != s.LastBuilt.Version {
		t.Fatalf("publish: %s\n%+v", body, s.Published)
	}
	if len(st.calls) != 2 {
		t.Fatalf("calls %v", st.calls)
	}
	if rec := a.do(t, "/api/shops/al-raj/publish", `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("publishing twice: %d", rec.Code)
	}
}

func TestAppTestBuildWarnsUnpublished(t *testing.T) {
	a, st, _ := releaseApp(t, true) // a dirty tree is fine for a test build
	body := a.do(t, "/api/shops/al-raj/build", `{"installer":true}`).Body.String()
	if !strings.Contains(body, "This shop has no release yet — the first install at a shop should be a Release") || !strings.Contains(body, resultMark) {
		t.Fatalf("test build: %s", body)
	}
	if s := a.stored(t); s.LastBuilt.Kind != "test" || s.Published != nil || len(st.calls) != 0 {
		t.Fatalf("test build must not publish: %+v %v", s.LastBuilt, st.calls)
	}
	if rec := a.do(t, "/api/shops/al-raj/publish", `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("publishing a test build: %d", rec.Code)
	}
}

func TestAppReleaseDirty(t *testing.T) {
	a, _, _ := releaseApp(t, true)
	if rec := a.do(t, "/api/shops/al-raj/release", `{"installer":true}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "commit first") {
		t.Fatalf("dirty release: %d %s", rec.Code, rec.Body)
	}
}

func TestAppReleaseAll(t *testing.T) {
	a, st, _ := releaseApp(t, false)
	a.do(t, "/api/shops", `{"key":"zed","name":"Zed","os":"windows","arch":"amd64","mode":"file"}`)
	body := a.do(t, "/api/shops/release-all", `{"installer":true,"notes":"n"}`).Body.String()
	shops, _ := LoadShops(a.path)
	if !strings.Contains(body, resultMark) || len(shops.Shops) != 2 || shops.Shops[0].Published == nil || shops.Shops[1].Published == nil ||
		shops.Shops[0].Published.Version != shops.Shops[1].Published.Version {
		t.Fatalf("release all: %s\n%+v", body, shops)
	}
	if len(st.calls) != 4 {
		t.Fatalf("calls %v", st.calls)
	}
}

// A Test build remembers whether the tree had uncommitted changes, so the row
// can tell "same code as published" from "code not released yet".
func TestAppTestBuildRecordsDirty(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		a, _, _ := releaseApp(t, dirty)
		a.do(t, "/api/shops/al-raj/build", `{"installer":false}`)
		if s := a.stored(t); s.LastBuilt == nil || s.LastBuilt.Dirty != dirty {
			t.Fatalf("dirty=%v: lastBuilt %+v", dirty, s.LastBuilt)
		}
	}
}
