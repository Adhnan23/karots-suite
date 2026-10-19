// Package shopconf is install.conf: what the Shop Builder writes beside a shop's
// binary and what karots-setup reads to install it. The Builder decides every
// value; the installer only applies them.
package shopconf

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Version is bumped when an older installer could misread the file.
const Version = 1

// FileName sits next to karots-setup and the binary.
const FileName = "install.conf"

// The PostgreSQL 17 installer for a Windows server-mode main PC — the one
// winget's PostgreSQL.PostgreSQL.17 (17.11-5) points at. The Builder can put it
// beside the three files (no download at the shop); Setup uses it from there,
// else winget, else downloads it. Bump all three together from the winget-pkgs
// manifest.
const (
	PGInstallerName   = "postgresql-17.11-5-windows-x64.exe"
	PGInstallerURL    = "https://get.enterprisedb.com/postgresql/" + PGInstallerName
	PGInstallerSHA256 = "612C7F5400A003AAFFA0372CB273EED52278F93C680AB911EB9CB2229E329072"
)

// DB is a server-mode shop's PostgreSQL.
type DB struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Name          string `json:"name"`
	User          string `json:"user"`
	Password      string `json:"password"`
	SuperPassword string `json:"superPassword"` // Windows: the postgres superuser Setup installs it with
	Local         bool   `json:"local"`         // the host is this PC
}

// Conf is one shop's install.
type Conf struct {
	Version    int               `json:"version"`
	Shop       string            `json:"shop"`
	Key        string            `json:"key"`
	InstallID  string            `json:"installId"`
	OS         string            `json:"os"`     // linux | windows
	Mode       string            `json:"mode"`   // file | server
	Binary     string            `json:"binary"` // file name beside the installer
	InstallDir string            `json:"installDir"`
	Port       int               `json:"port"`
	Open       string            `json:"open"` // kiosk | background
	LanAddress string            `json:"lanAddress,omitempty"`
	DB         *DB               `json:"db,omitempty"` // nil in file mode
	Env        map[string]string `json:"env"`          // the final .env
}

// Read loads and checks install.conf.
func Read(path string) (Conf, error) {
	var c Conf
	raw, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, c.check()
}

func (c Conf) check() error {
	switch {
	case c.Version != Version:
		return fmt.Errorf("install.conf version %d: this installer reads version %d — rebuild in the Shop Builder", c.Version, Version)
	case c.Mode != "file" && c.Mode != "server":
		return fmt.Errorf("install.conf: mode %q (file | server)", c.Mode)
	case c.Mode == "server" && c.DB == nil:
		return fmt.Errorf("install.conf: a server-mode shop needs db")
	case c.Binary == "" || strings.ContainsAny(c.Binary, `/\`) || strings.HasPrefix(c.Binary, "."):
		return fmt.Errorf("install.conf: binary %q must be a plain file name", c.Binary)
	case c.Port < 1 || c.Port > 65535:
		return fmt.Errorf("install.conf: port %d", c.Port)
	case c.Open != "kiosk" && c.Open != "background":
		return fmt.Errorf("install.conf: open %q (kiosk | background)", c.Open)
	case c.InstallDir == "":
		return fmt.Errorf("install.conf: installDir is empty")
	}
	for k, v := range c.Env {
		if k == "" || strings.ContainsAny(k, "= \t\r\n#") || strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("install.conf: env %q is not a KEY=value line", k)
		}
	}
	return nil
}

// Write saves c owner-only: it holds the shop's secrets.
func Write(path string, c Conf) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600) // WriteFile keeps an existing file's mode
}

// EnvFile is the .env: KEY=value lines, sorted, no quotes or comments, so a
// systemd EnvironmentFile reads it as the app does.
func (c Conf) EnvFile() []byte {
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + c.Env[k] + "\n")
	}
	return []byte(b.String())
}
