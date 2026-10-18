package filedb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// testBundle downloads this machine's Postgres once into the user cache (or uses
// the one already there), so the tests run without a file-mode build.
func testBundle(t *testing.T) []byte {
	t.Helper()
	if testing.Short() {
		t.Skip("starts a real PostgreSQL")
	}
	if !Supported(runtime.GOOS, runtime.GOARCH) {
		t.Skip("no Postgres bundle for this platform")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip(err)
	}
	path, err := FetchBundle(filepath.Join(cache, "karots-suite-filedb"), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("no bundle (offline?): %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func open(t *testing.T, dir string, bundle []byte) *DB {
	t.Helper()
	d, err := Open(Options{Dir: dir, Port: 54392, Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestOpenKeepsDataAndSurvivesALeftover(t *testing.T) {
	bundle := testBundle(t)
	dir := t.TempDir()

	d := open(t, dir, bundle)
	db := sqlx.MustConnect("postgres", d.URL)
	db.MustExec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`)
	db.MustExec(`CREATE TABLE p (name text)`)
	db.MustExec(`INSERT INTO p VALUES ('සීනි කිලෝ 1')`)
	var hits int
	if err := db.Get(&hits, `SELECT count(*) FROM p WHERE similarity(name, 'සීනි') > 0.2`); err != nil || hits != 1 {
		t.Fatalf("Sinhala trigram search: hits=%d err=%v (a plain C locale gives 0)", hits, err)
	}
	db.Close()
	jwt := d.JWTSecret

	// A crash: the app is gone but its Postgres keeps running. Forget the handle
	// (never Stop it) and open again — the leftover must be stopped, data kept.
	d2 := open(t, dir, bundle)
	defer d2.Stop()
	db2 := sqlx.MustConnect("postgres", d2.URL)
	defer db2.Close()
	var n int
	if err := db2.Get(&n, `SELECT count(*) FROM p`); err != nil || n != 1 {
		t.Fatalf("data after restart: n=%d err=%v", n, err)
	}
	if d2.JWTSecret != jwt || len(jwt) != 64 {
		t.Fatalf("JWT secret must be generated once and kept: %q vs %q", jwt, d2.JWTSecret)
	}
}

func TestOpenRefusesAFolderItWouldWipe(t *testing.T) {
	for name, pgVersion := range map[string]string{"other major": "16\n", "not postgres": ""} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			data := filepath.Join(dir, "pg")
			if err := os.MkdirAll(data, 0o755); err != nil {
				t.Fatal(err)
			}
			keep := filepath.Join(data, "base")
			os.WriteFile(keep, []byte("x"), 0o644)
			if pgVersion != "" {
				os.WriteFile(filepath.Join(data, "PG_VERSION"), []byte(pgVersion), 0o644)
			}
			if _, err := Open(Options{Dir: dir, Port: 54393, Bundle: []byte("x")}); err == nil {
				t.Fatal("expected a refusal")
			}
			if _, err := os.Stat(keep); err != nil {
				t.Fatal("the folder was touched")
			}
		})
	}
}

func TestNoBundleNoFileMode(t *testing.T) {
	if Available() {
		t.Skip("file-mode build")
	}
	if _, err := Open(Options{Dir: t.TempDir()}); err == nil {
		t.Fatal("a server-only build must refuse file mode")
	}
}
