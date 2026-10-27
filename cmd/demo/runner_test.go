package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"karots-suite/internal/db"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/filedb"
)

func TestWipeDataKeepsPrograms(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"pg/postmaster.pid", "secret", "postgres.log", "pg-17/bin/postgres"} {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, n)), 0o700)
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600)
	}
	if err := wipeData(dir); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"pg", "secret", "postgres.log"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s survived", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "pg-17/bin/postgres")); err != nil {
		t.Error("PostgreSQL's programs were removed")
	}
}

func TestSiblingURL(t *testing.T) {
	got := siblingURL("postgres://shop:pw@127.0.0.1:5999/shop?sslmode=disable", "shop_3")
	if got != "postgres://shop:pw@127.0.0.1:5999/shop_3?sslmode=disable" {
		t.Fatal(got)
	}
}

// realRunner starts the launcher's PostgreSQL and builds this command, so the
// runner starts real children. Slow; skipped with -short or without a bundle.
func realRunner(t *testing.T) *childRunner {
	t.Helper()
	if testing.Short() {
		t.Skip("starts PostgreSQL and real shops")
	}
	cache, _ := os.UserCacheDir()
	path, err := filedb.FetchBundle(filepath.Join(cache, "karots-suite-filedb"), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no PostgreSQL bundle: %v", err)
	}
	bundle, _ := os.ReadFile(path)
	dir := t.TempDir()
	fdb, err := filedb.Open(filedb.Options{Dir: dir, Port: freePort(), Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fdb.Stop() })
	pg, err := db.Connect(fdb.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	exe := filepath.Join(dir, "karots-demo")
	if out, err := exec.Command("go", "build", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	r := &childRunner{exe: exe, pg: pg, baseURL: fdb.URL, receipt: "127.0.0.1:9", label: "127.0.0.1:9", logOutput: os.Stderr}
	t.Cleanup(func() { r.Stop() })
	return r
}

func databases(t *testing.T, r *childRunner) []string {
	var names []string
	if err := r.pg.Select(&names, `SELECT datname FROM pg_database WHERE datname LIKE 'shop\_%' ORDER BY 1`); err != nil {
		t.Fatal(err)
	}
	return names
}

func TestSwitchLeavesOneChild(t *testing.T) {
	r := realRunner(t)
	grocery, _ := shops.ByKey("grocery")
	pharmacy, _ := shops.ByKey("pharmacy")
	nop := func(int, string) {}
	if err := r.Start(grocery, nop); err != nil {
		t.Fatal(err)
	}
	first := r.cmd
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(pharmacy, nop); err != nil {
		t.Fatal(err)
	}
	if first.ProcessState == nil {
		t.Fatal("the first shop's process is still running")
	}
	if got := databases(t, r); len(got) != 1 || got[0] != "shop_2" {
		t.Fatalf("databases = %v, want [shop_2]", got)
	}
	resp, err := http.Get(r.URL() + "/api/v1/auth/login-info")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("the pharmacy doesn't answer: %v", err)
	}
	resp.Body.Close()
}

// A sign-in from one shop must not open the next one.
func TestEachShopGetsItsOwnSigningSecret(t *testing.T) {
	r := &childRunner{baseURL: "postgres://shop:pw@127.0.0.1:1/shop?sslmode=disable"}
	p, _ := shops.ByKey("grocery")
	a, b := r.argsFor("shop_1", p, 1, 2), r.argsFor("shop_2", p, 3, 4)
	if a.JWTSecret == b.JWTSecret || len(a.JWTSecret) < 32 {
		t.Fatalf("secrets %q / %q", a.JWTSecret, b.JWTSecret)
	}
}

// A demo killed with its PostgreSQL still running (a closed window) must stop
// that server before wiping its folder, or it lives on — and on Windows holds
// files so the next start can't clear them.
func TestPrepareStopsALeftoverPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("starts PostgreSQL")
	}
	cache, _ := os.UserCacheDir()
	path, err := filedb.FetchBundle(filepath.Join(cache, "karots-suite-filedb"), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no PostgreSQL bundle: %v", err)
	}
	bundle, _ := os.ReadFile(path)
	dir := t.TempDir()
	port := freePort()
	if _, err := filedb.Open(filedb.Options{Dir: dir, Port: port, Bundle: bundle}); err != nil { // never stopped: the crash
		t.Fatal(err)
	}
	if err := prepareDir(dir); err != nil {
		t.Fatal(err)
	}
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
		c.Close()
		t.Fatal("the old PostgreSQL is still running after prepareDir")
	}
	if _, err := os.Stat(filepath.Join(dir, "pg")); !os.IsNotExist(err) {
		t.Fatal("the old data survived")
	}
}
