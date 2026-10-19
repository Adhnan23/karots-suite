package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/update"
)

// fakeStore records uploads and serves them back like the public bucket.
type fakeStore struct {
	calls []string // "put <key> <cache>" / "delete <key>"
	files map[string][]byte
}

func newFakeStore(t *testing.T) (*fakeStore, string) {
	f := &fakeStore{files: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := f.files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func (f *fakeStore) put(key, path, _, cache string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f.calls = append(f.calls, "put "+key+" "+cache)
	f.files[key] = b
	return nil
}

func (f *fakeStore) delete(key string) error {
	f.calls = append(f.calls, "delete "+key)
	delete(f.files, key)
	return nil
}

// builtShop is a shop with a binary in dir, as a release build leaves it.
func builtShop(t *testing.T) (Shop, string) {
	t.Helper()
	s := mustShop(t, "al-raj", "linux", "file")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, shopBinary), []byte("binary "+s.Key), 0o755)
	return s, dir
}

func TestPublishOrder(t *testing.T) {
	st, base := newFakeStore(t)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, dir := builtShop(t)
	if err := publish(&s, dir, "2026.10.19.1530", "faster till", "abc1234", base, st, priv, io.Discard); err != nil {
		t.Fatal(err)
	}
	gz := s.Channel + "/karots-suite-2026.10.19.1530.gz"
	want := []string{"put " + gz + " ", "put " + s.Channel + "/manifest.json no-store"}
	if strings.Join(st.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls %v, want %v", st.calls, want)
	}
	var m update.Manifest
	if err := json.Unmarshal(st.files[s.Channel+"/manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	if err := m.Verify(pub); err != nil || m.Notes != "faster till" || m.Commit != "abc1234" || m.File != "karots-suite-2026.10.19.1530.gz" {
		t.Fatalf("manifest %+v: %v", m, err)
	}
	if s.Published == nil || s.Published.Version != "2026.10.19.1530" || s.Published.Commit != "abc1234" || s.Published.At.IsZero() {
		t.Fatalf("published = %+v", s.Published)
	}
}

func TestPublishPrunes(t *testing.T) {
	st, base := newFakeStore(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, dir := builtShop(t)
	for i, v := range []string{"2026.10.19.1500", "2026.10.19.1510", "2026.10.19.1520", "2026.10.19.1530"} {
		if err := publish(&s, dir, v, "release "+v, "c"+v, base, st, priv, io.Discard); err != nil {
			t.Fatalf("release %d: %v", i, err)
		}
	}
	var m update.Manifest
	json.Unmarshal(st.files[s.Channel+"/manifest.json"], &m)
	if len(m.History) != 3 || m.History[0] != "karots-suite-2026.10.19.1530.gz" {
		t.Fatalf("history %v", m.History)
	}
	if _, ok := st.files[s.Channel+"/karots-suite-2026.10.19.1500.gz"]; ok {
		t.Fatal("the oldest binary should be deleted")
	}
	if len(m.Changelog) != 4 || m.Changelog[0].Version != "2026.10.19.1530" {
		t.Fatalf("changelog %+v", m.Changelog)
	}
}

func TestPublishNotNewer(t *testing.T) {
	st, base := newFakeStore(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, dir := builtShop(t)
	publish(&s, dir, "2026.10.19.1530", "", "c1", base, st, priv, io.Discard)
	st.calls = nil
	err := publish(&s, dir, "2026.10.19.1530", "", "c2", base, st, priv, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "is not newer than the published") || len(st.calls) != 0 {
		t.Fatalf("err %v, calls %v", err, st.calls)
	}
}

func TestReleaseKeyGenerates(t *testing.T) {
	env := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(env, []byte("APP_ENV=development\nSUITE_RELEASE_KEY=\nR2_BUCKET=karots-suite\n"), 0o600)
	t.Setenv("SUITE_RELEASE_KEY", "")
	_, pub1, err := releaseKey(env, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(env)
	if strings.Count(string(b), "SUITE_RELEASE_KEY=") != 1 || strings.Contains(string(b), "SUITE_RELEASE_KEY=\n") || !strings.Contains(string(b), "R2_BUCKET=karots-suite") {
		t.Fatalf(".env after:\n%s", b)
	}
	_, pub2, err := releaseKey(env, io.Discard)
	if err != nil || pub1 != pub2 {
		t.Fatalf("second call: %v, %q vs %q", err, pub2, pub1)
	}
	if raw, _ := base64.StdEncoding.DecodeString(pub1); len(raw) != ed25519.PublicKeySize {
		t.Fatalf("public key %q", pub1)
	}
}

func TestTreeDirtyIgnoresBuilderFiles(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q")
	for _, f := range append([]string{"main.go"}, builderWrites...) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755)
		os.WriteFile(filepath.Join(dir, f), []byte("a"), 0o644)
	}
	git("add", ".")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "x")
	for _, f := range builderWrites {
		os.WriteFile(filepath.Join(dir, f), []byte("b"), 0o644)
	}
	if dirty, err := treeDirty(dir); dirty || err != nil {
		t.Fatalf("only Builder files changed: dirty=%v err=%v", dirty, err)
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("b"), 0o644)
	if dirty, _ := treeDirty(dir); !dirty {
		t.Fatal("a changed source file must count as dirty")
	}
}

func TestShopBuildBakesVersion(t *testing.T) {
	out := t.TempDir()
	s := mustShop(t, "al-raj", "linux", "server")
	t.Setenv("UPDATE_BASE_URL", "https://pub-x.r2.dev")
	var got buildOpts
	if _, err := buildShop(&s, nil, shopBuildOpts{Out: out, Version: "2026.10.19.1530", Kind: "test", PubKey: "PUBKEY"}, fakeBuild(&got), fakeSetup, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, x := range []string{"main.version=2026.10.19.1530", "main.buildKind=test", "main.updateBaseURL=https://pub-x.r2.dev", "main.updateChannel=" + s.Channel, "main.updatePubKey=PUBKEY"} {
		if !strings.Contains(got.ExtraX, x) {
			t.Errorf("ExtraX %q lacks %q", got.ExtraX, x)
		}
	}
	if s.LastBuilt == nil || s.LastBuilt.Version != "2026.10.19.1530" || s.LastBuilt.Kind != "test" {
		t.Fatalf("lastBuilt = %+v", s.LastBuilt)
	}
}

func TestVersionAt(t *testing.T) {
	if v := versionAt(time.Date(2026, 10, 9, 7, 5, 0, 0, time.Local)); v != "2026.10.09.0705" {
		t.Fatalf("versionAt = %q", v)
	}
}
