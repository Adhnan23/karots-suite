// Package filedb runs the shop's own PostgreSQL from a folder next to the binary
// (file mode): no database to install, no .env to write. It is a real Postgres
// (the suite's SQL is Postgres-only), started through embedded-postgres from the
// bundle baked into a `-tags filemode` build — never downloaded at the shop.
//
// Unlike Demo Room's throwaway database it keeps its data: a restart reuses the
// folder, a Postgres left running by a crash is stopped first, and a folder it
// can't use is refused rather than wiped.
package filedb

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

// DefaultPort sits next to dev's 5433 and clear of a system Postgres on 5432.
const DefaultPort = 5434

type Options struct {
	Dir    string // the data folder (data/ next to the binary)
	Port   int
	Bundle []byte // the Postgres .txz; nil = the one baked into this build
}

type DB struct {
	Dir       string
	URL       string // DATABASE_URL for the app
	JWTSecret string // generated once, kept in Dir/secret
	pg        *embeddedpostgres.EmbeddedPostgres
}

// Open starts the shop's Postgres, creating the database on first use.
func Open(o Options) (*DB, error) {
	if o.Bundle == nil {
		o.Bundle = bundled
	}
	if len(o.Bundle) == 0 {
		return nil, errors.New("this build has no PostgreSQL of its own — set DATABASE_URL, or build it in file mode")
	}
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		return nil, errors.New("file mode can't run as root (PostgreSQL refuses it) — run the shop as a normal user")
	}
	if o.Port == 0 {
		o.Port = DefaultPort
	}
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	data := filepath.Join(dir, "pg")
	if err := checkDataDir(data); err != nil {
		return nil, err
	}
	cache := filepath.Join(dir, "cache")
	if err := placeBundle(cache, o.Bundle); err != nil {
		return nil, fmt.Errorf("unpack PostgreSQL: %w", err)
	}
	stopLeftover(dir, data)
	sec, err := secrets(filepath.Join(dir, "secret"))
	if err != nil {
		return nil, err
	}
	logf, err := os.OpenFile(filepath.Join(dir, "postgres.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.PostgresVersion(PGVersion)).
		Port(uint32(o.Port)).
		Username("shop").Password(sec["FILE_DB_PASSWORD"]).Database("shop").
		RuntimePath(filepath.Join(dir, "runtime")).
		DataPath(data).
		CachePath(cache).
		Encoding("UTF8").
		StartParameters(map[string]string{"listen_addresses": "127.0.0.1"}).
		StartTimeout(60 * time.Second).
		Logger(logf)
	if loc := locale(); loc != "" {
		cfg = cfg.Locale(loc)
	}
	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		logf.Close()
		if strings.Contains(err.Error(), "already listening") {
			return nil, fmt.Errorf("port %d is taken by another program — set FILE_DB_PORT in .env to a free port", o.Port)
		}
		// 0xc0000135 = Windows "DLL not found": zonky's PostgreSQL needs the VC++ runtime.
		if strings.Contains(err.Error(), "0xc0000135") {
			return nil, fmt.Errorf("PostgreSQL needs the Microsoft Visual C++ runtime — install https://aka.ms/vs/17/release/vc_redist.x64.exe, delete %s, then start again", dir)
		}
		return nil, fmt.Errorf("start PostgreSQL (see %s): %w", filepath.Join(dir, "postgres.log"), err)
	}
	return &DB{
		Dir:       dir,
		URL:       fmt.Sprintf("postgres://shop:%s@127.0.0.1:%d/shop?sslmode=disable", sec["FILE_DB_PASSWORD"], o.Port),
		JWTSecret: sec["JWT_SECRET"],
		pg:        pg,
	}, nil
}

// Stop shuts Postgres down cleanly.
func (d *DB) Stop() error { return d.pg.Stop() }

// locale keeps Sinhala / Tamil search working: plain C makes pg_trgm treat every
// non-ASCII letter as punctuation (similarity 0). Windows' default locale is fine.
func locale() string {
	switch runtime.GOOS {
	case "linux":
		return "C.UTF-8"
	case "darwin":
		return "en_US.UTF-8"
	}
	return ""
}

// checkDataDir refuses a folder embedded-postgres would otherwise wipe and re-init:
// one from another Postgres major, or one that isn't a Postgres folder at all.
func checkDataDir(data string) error {
	entries, err := os.ReadDir(data)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(entries) == 0) {
		return nil // first run
	}
	if err != nil {
		return err
	}
	v, err := os.ReadFile(filepath.Join(data, "PG_VERSION"))
	if err != nil {
		return fmt.Errorf("%s isn't a PostgreSQL data folder — move it away (or restore a backup into a fresh one)", data)
	}
	if got, want := strings.TrimSpace(string(v)), strings.Split(PGVersion, ".")[0]; got != want {
		return fmt.Errorf("%s was made by PostgreSQL %s but this build runs %s — take a backup with the old build, then restore it into this one", data, got, want)
	}
	return nil
}

// placeBundle puts the bundle where embedded-postgres looks before downloading.
func placeBundle(cache string, bundle []byte) error {
	path := filepath.Join(cache, BundleName(runtime.GOOS, runtime.GOARCH))
	if fi, err := os.Stat(path); err == nil && fi.Size() == int64(len(bundle)) {
		return nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path+".part", bundle, 0o644); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

// stopLeftover stops a Postgres still running on this folder after the app was
// killed (a crash, a closed window): otherwise its port is busy and start fails.
// The caller has already claimed the app's own port, so this is never a live shop.
func stopLeftover(dir, data string) {
	if _, err := os.Stat(filepath.Join(data, "postmaster.pid")); err != nil {
		return
	}
	pgctl := filepath.Join(dir, "runtime", "bin", "pg_ctl")
	if runtime.GOOS == "windows" {
		pgctl += ".exe"
	}
	if _, err := os.Stat(pgctl); err != nil {
		return
	}
	_ = exec.Command(pgctl, "stop", "-D", data, "-m", "fast", "-w").Run() // "no server running" is fine
}

// secrets reads Dir/secret (KEY=value lines), creating it once with random values.
func secrets(path string) (map[string]string, error) {
	s := map[string]string{}
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if k, v, ok := strings.Cut(sc.Text(), "="); ok {
				s[k] = v
			}
		}
		f.Close()
	}
	if s["JWT_SECRET"] != "" && s["FILE_DB_PASSWORD"] != "" {
		return s, nil
	}
	for _, k := range []string{"JWT_SECRET", "FILE_DB_PASSWORD"} {
		if s[k] == "" {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			s[k] = hex.EncodeToString(b)
		}
	}
	body := "JWT_SECRET=" + s["JWT_SECRET"] + "\nFILE_DB_PASSWORD=" + s["FILE_DB_PASSWORD"] + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return s, os.WriteFile(path, []byte(body), 0o600)
}
