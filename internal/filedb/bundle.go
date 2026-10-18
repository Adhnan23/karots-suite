package filedb

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// PGVersion is the PostgreSQL every file-mode shop runs. Changing the major
// version needs a backup → restore move (the data folder is refused, never wiped).
const PGVersion = "17.5.0"

// bundled is the Postgres bundle (.txz) baked into a file-mode build
// (embed_filemode.go); nil in a server-only build.
var bundled []byte

// Available reports whether this binary carries its own Postgres.
func Available() bool { return len(bundled) > 0 }

// bundleArch is zonky's name for a CPU (embedded-postgres maps it the same way).
func bundleArch(goos, goarch string) string {
	if goos == "linux" && goarch == "arm64" {
		return "arm64v8"
	}
	return goarch
}

// BundleName is the cache file name embedded-postgres looks for.
func BundleName(goos, goarch string) string {
	return fmt.Sprintf("embedded-postgres-binaries-%s-%s-%s.txz", goos, bundleArch(goos, goarch), PGVersion)
}

// Supported reports whether a Postgres bundle exists for that OS / CPU.
func Supported(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin":
		return goarch == "amd64" || goarch == "arm64"
	case "windows":
		return goarch == "amd64"
	}
	return false
}

// FetchBundle returns the path of the .txz for goos/goarch in cacheDir, downloading
// it (from Maven Central, SHA-256 checked) only when it isn't there yet. Used by the
// Shop Builder (any target OS) and by the tests.
func FetchBundle(cacheDir, goos, goarch string) (string, error) {
	if !Supported(goos, goarch) {
		return "", fmt.Errorf("file mode has no PostgreSQL for %s/%s (linux, macOS amd64/arm64, windows amd64)", goos, goarch)
	}
	path := filepath.Join(cacheDir, BundleName(goos, goarch))
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	arch := bundleArch(goos, goarch)
	url := fmt.Sprintf("https://repo1.maven.org/maven2/io/zonky/test/postgres/embedded-postgres-binaries-%s-%s/%s/embedded-postgres-binaries-%s-%s-%s.jar",
		goos, arch, PGVersion, goos, arch, PGVersion)
	jar, err := get(url)
	if err != nil {
		return "", err
	}
	want, err := get(url + ".sha256")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(jar)
	if hex.EncodeToString(sum[:]) != strings.TrimSpace(string(want)) {
		return "", fmt.Errorf("PostgreSQL download for %s/%s failed its SHA-256 check", goos, goarch)
	}
	zr, err := zip.NewReader(bytes.NewReader(jar), int64(len(jar)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".txz") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		if err := os.MkdirAll(cacheDir, 0o755); err != nil {
			return "", err
		}
		tmp := path + ".part"
		out, err := os.Create(tmp)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return path, os.Rename(tmp, path)
	}
	return "", fmt.Errorf("no .txz inside %s", url)
}

func get(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
