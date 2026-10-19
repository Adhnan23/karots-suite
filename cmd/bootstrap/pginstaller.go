package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"karots-suite/internal/shopconf"
)

// The PostgreSQL installer a Windows main PC needs, from shopconf; variables so
// tests can serve their own.
var (
	pgURL    = shopconf.PGInstallerURL
	pgSHA256 = shopconf.PGInstallerSHA256
	pgCache  = func() (string, error) {
		d, err := os.UserCacheDir()
		return filepath.Join(d, "karots-suite-builder"), err
	}
)

// needsPG: a Windows main PC that installs its own PostgreSQL (server mode,
// database on the same PC) — the shop that gets the installer as a 4th file.
func needsPG(s Shop) bool {
	return s.OS == "windows" && s.Mode == "server" && s.DB != nil && isLocalHost(s.DB.Host)
}

// pgInstaller is the cached PostgreSQL installer, downloaded once (413 MB) on
// the developer's PC and checked against its pinned SHA-256, so a shop on a slow
// or metered line never downloads it.
func pgInstaller(log io.Writer) (string, error) {
	dir, err := pgCache()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, shopconf.PGInstallerName)
	if sum, err := fileSHA256(path); err == nil && strings.EqualFold(sum, pgSHA256) {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	fmt.Fprintf(log, "→ downloading the PostgreSQL installer once (413 MB) into %s\n", dir)
	res, err := http.Get(pgURL)
	if err != nil {
		return "", fmt.Errorf("download the PostgreSQL installer: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download the PostgreSQL installer: %s", res.Status)
	}
	part := path + ".part"
	f, err := os.Create(part)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), res.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), pgSHA256) {
		err = errors.New("the PostgreSQL installer's checksum differs from the pinned one (damaged download?)")
	}
	if err != nil {
		os.Remove(part)
		return "", err
	}
	return path, os.Rename(part, path)
}

// linkOrCopy puts src at dst: a hard link when the disk allows (no 413 MB copy
// per build), else a copy.
func linkOrCopy(src, dst string) error {
	os.Remove(dst)
	if os.Link(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
