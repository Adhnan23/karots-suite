package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/filedb"
	"karots-suite/internal/shopconf"
	"karots-suite/internal/support"
)

// A shop profile in shops.json: everything needed to build that shop again,
// exactly as last time, and install it. It holds the shop's secrets, so the
// repo it lives in stays private.

type ShopDB struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Name          string `json:"name"`
	User          string `json:"user"`
	Password      string `json:"password"`
	SuperPassword string `json:"superPassword"` // Windows: the postgres superuser Setup installs PostgreSQL with
	// SSL is the connection's sslmode: disable | require | verify-full. Empty =
	// off for this PC (localhost), required for anything else (a VPS, Neon…).
	SSL string `json:"ssl,omitempty"`
}

// sslMode is the sslmode a DATABASE_URL gets.
func (d ShopDB) sslMode() string {
	if d.SSL != "" {
		return d.SSL
	}
	if isLocalHost(d.Host) {
		return "disable"
	}
	return "require"
}

func isLocalHost(h string) bool { return h == "localhost" || h == "127.0.0.1" || h == "::1" }

type ShopInstall struct {
	Dir        string `json:"dir"`
	Open       string `json:"open"`                 // kiosk | background
	LanAddress string `json:"lanAddress,omitempty"` // pre-fills a cashier PC's main-PC address
}

// ShopBackups is how the shop's automatic backups run. Empty = the defaults:
// <install folder>/backups (one PC: data/backups), every 6 h, keep 28.
type ShopBackups struct {
	Dir   string `json:"dir,omitempty"`
	Every string `json:"every,omitempty"` // a Go duration: 1h, 6h, 24h
	Keep  int    `json:"keep,omitempty"`
}

const (
	defaultBackupEvery = "6h"
	defaultBackupKeep  = 28
)

type Built struct {
	At      time.Time `json:"at"`
	Commit  string    `json:"commit"`
	Version string    `json:"version,omitempty"`
	Kind    string    `json:"kind,omitempty"`  // "test" (local only) | "release" (meant for the bucket)
	Notes   string    `json:"notes,omitempty"` // a release's "what's new", kept for a Publish retry
}

type Shop struct {
	Key        string            `json:"key"`
	Name       string            `json:"name"`
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	Mode       string            `json:"mode"` // file | server
	Plugins    []string          `json:"plugins"`
	InstallID  string            `json:"installId"` // never changes: the shop's support PIN hangs off it
	Channel    string            `json:"channel"`   // its update folder (M8c)
	Port       int               `json:"port"`
	DB         *ShopDB           `json:"db,omitempty"`         // server mode
	FileDBPort int               `json:"fileDbPort,omitempty"` // file mode
	JWTSecret  string            `json:"jwtSecret,omitempty"`  // server mode (file mode keeps its own in data/)
	Install    ShopInstall       `json:"install"`
	Backups    ShopBackups       `json:"backups"`
	Env        map[string]string `json:"env,omitempty"` // extra .env keys
	LastBuilt  *Built            `json:"lastBuilt,omitempty"`
	Published  *Published        `json:"published,omitempty"` // what its bucket folder holds (M8c)
}

type Shops struct {
	Shops []Shop `json:"shops"`
}

const (
	defaultPort       = 3000
	defaultDBPort     = 5432
	defaultFileDBPort = filedb.DefaultPort
	maskedValue       = "••••"
)

var (
	shopKeyFmt = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	secretKey  = regexp.MustCompile(`SECRET|PASSWORD|KEY|TOKEN`)
	// Keys a shop's env may not carry: the support secret is baked, never shipped;
	// a fixed PIN defeats the rotation; the rest come from the profile's fields.
	refusedEnv = []string{"POS_SUPPORT_SECRET", "POS_SYSTEM_PIN", "APP_ENV", "SERVER_PORT", "DATABASE_URL", "JWT_SECRET", "BACKUP_DIR", "BACKUP_INTERVAL", "BACKUP_KEEP", "FILE_DB_PORT"}
)

// LoadShops reads path; a missing file is an empty list.
func LoadShops(path string) (Shops, error) {
	var s Shops
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// SaveShops writes path sorted by key, through a temp file in the same folder
// and a rename, so a crash mid-save never loses every shop.
func SaveShops(path string, s Shops) error {
	sort.Slice(s.Shops, func(i, j int) bool { return s.Shops[i].Key < s.Shops[j].Key })
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".shops-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func randHex(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on a supported OS
	}
	return hex.EncodeToString(b)
}

// NewShop is a fresh profile with its install id, channel and secrets minted.
func NewShop(key, name, osName, arch, mode string) (Shop, error) {
	id, err := support.NewInstallID()
	if err != nil {
		return Shop{}, err
	}
	s := Shop{Key: key, Name: name, OS: osName, Arch: arch, Plugins: []string{}, InstallID: id, Channel: randHex(16), Port: defaultPort}
	s.Install = ShopInstall{Dir: "/opt/karots-suite", Open: "kiosk"}
	if osName == "windows" {
		s.Install.Dir = `C:\karots-suite`
	}
	return s, s.SetMode(mode)
}

// SetMode switches the database: file drops the server's DB and JWT secret,
// server mints them.
func (s *Shop) SetMode(mode string) error {
	switch mode {
	case "file":
		s.DB, s.JWTSecret = nil, ""
		if s.FileDBPort == 0 {
			s.FileDBPort = defaultFileDBPort
		}
	case "server":
		s.FileDBPort = 0
		if s.DB == nil {
			s.DB = &ShopDB{Host: "localhost", Port: defaultDBPort, Name: "suite_db", User: "suite_user", Password: randHex(16), SuperPassword: randHex(16)}
		}
		if s.JWTSecret == "" {
			s.JWTSecret = randHex(24)
		}
	default:
		return fmt.Errorf("unsupported mode %q (server | file)", mode)
	}
	s.Mode = mode
	return nil
}

// Rotate makes a new value for one secret: jwt | db | dbsuper.
func (s *Shop) Rotate(what string) error {
	switch {
	case what == "jwt" && s.Mode == "server":
		s.JWTSecret = randHex(24)
	case what == "db" && s.DB != nil:
		s.DB.Password = randHex(16)
	case what == "dbsuper" && s.DB != nil:
		s.DB.SuperPassword = randHex(16)
	default:
		return fmt.Errorf("nothing to rotate for %q on a %s-mode shop", what, s.Mode)
	}
	return nil
}

// Check refuses a profile that can't be built; warnings are allowed.
func (s Shop) Check(all []manifest) (warnings []string, err error) {
	switch {
	case !shopKeyFmt.MatchString(s.Key):
		return nil, fmt.Errorf("shop key %q: lowercase letters, digits and dashes (e.g. al-raj)", s.Key)
	case strings.TrimSpace(s.Name) == "":
		return nil, fmt.Errorf("%s: the shop needs a name", s.Key)
	case s.OS != "linux" && s.OS != "windows":
		return nil, fmt.Errorf("%s: shops run Linux or Windows, not %q", s.Key, s.OS)
	case !validArch[s.Arch]:
		return nil, fmt.Errorf("%s: unsupported architecture %q", s.Key, s.Arch)
	case s.Mode == "file" && !filedb.Supported(s.OS, s.Arch):
		return nil, fmt.Errorf("%s: file mode has no PostgreSQL for %s/%s", s.Key, s.OS, s.Arch)
	case s.Mode == "server" && (s.DB == nil || s.JWTSecret == ""):
		return nil, fmt.Errorf("%s: a server-mode shop needs its database and JWT secret", s.Key)
	case s.DB != nil && s.DB.SSL != "" && s.DB.SSL != "disable" && s.DB.SSL != "require" && s.DB.SSL != "verify-full":
		return nil, fmt.Errorf("%s: database SSL %q (disable | require | verify-full)", s.Key, s.DB.SSL)
	case s.Mode != "server" && s.Mode != "file":
		return nil, fmt.Errorf("%s: unsupported mode %q", s.Key, s.Mode)
	case s.Install.Open != "kiosk" && s.Install.Open != "background":
		return nil, fmt.Errorf("%s: open %q (kiosk | background)", s.Key, s.Install.Open)
	case strings.TrimSpace(s.Install.Dir) == "":
		return nil, fmt.Errorf("%s: the install folder is empty", s.Key)
	}
	if d := s.Backups.Dir; d != "" && !(s.OS == "windows" && len(d) >= 3 && d[1] == ':' && (d[2] == '\\' || d[2] == '/') || s.OS != "windows" && strings.HasPrefix(d, "/")) {
		return nil, fmt.Errorf("%s: the backup folder must be a full path (e.g. %s)", s.Key, map[bool]string{true: `D:\Karots backups`, false: "/home/backups/karots"}[s.OS == "windows"])
	}
	if e := s.Backups.Every; e != "" {
		if d, err := time.ParseDuration(e); err != nil || d < 30*time.Minute {
			return nil, fmt.Errorf("%s: backups every %q — use 1h, 6h, 24h… (at least 30m)", s.Key, e)
		}
	}
	if s.Backups.Keep < 0 || s.Backups.Keep > 999 {
		return nil, fmt.Errorf("%s: keep %d backups — 1 to 999", s.Key, s.Backups.Keep)
	}
	ports := map[string]int{"till port": s.Port}
	if s.DB != nil {
		ports["database port"] = s.DB.Port
	} else {
		ports["file database port"] = s.FileDBPort
	}
	for what, p := range ports {
		if p < 1 || p > 65535 {
			return nil, fmt.Errorf("%s: %s %d is not a port", s.Key, what, p)
		}
	}
	if s.DB != nil && s.DB.Port == s.Port && s.Conf("").DB.Local {
		warnings = append(warnings, fmt.Sprintf("the till and its database both use port %d", s.Port))
	}
	if s.DB == nil && s.FileDBPort == s.Port {
		warnings = append(warnings, fmt.Sprintf("the till and its file database both use port %d", s.Port))
	}
	for k, v := range s.Env {
		if slices.Contains(refusedEnv, k) {
			return nil, fmt.Errorf("%s: %s can't be set in More settings", s.Key, k)
		}
		if k == "" || strings.ContainsAny(k, "= \t\r\n#") || strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("%s: setting %q is not a KEY=value line", s.Key, k)
		}
	}
	known := map[string]bool{}
	for _, m := range all {
		known[m.Key] = true
	}
	for _, p := range s.Plugins {
		if !known[p] {
			return nil, fmt.Errorf("%s: unknown plugin %q", s.Key, p)
		}
	}
	return warnings, nil
}

// Conf is the shop's install.conf: the install settings and the final .env.
func (s Shop) Conf(binary string) shopconf.Conf {
	c := shopconf.Conf{
		Version: shopconf.Version, Shop: s.Name, Key: s.Key, InstallID: s.InstallID, OS: s.OS, Mode: s.Mode,
		Binary: binary, InstallDir: s.Install.Dir, Port: s.Port, Open: s.Install.Open, LanAddress: s.Install.LanAddress,
		Env: map[string]string{"APP_ENV": "production", "SERVER_PORT": strconv.Itoa(s.Port)},
	}
	if s.DB != nil {
		local := isLocalHost(s.DB.Host)
		c.DB = &shopconf.DB{Host: s.DB.Host, Port: s.DB.Port, Name: s.DB.Name, User: s.DB.User, Password: s.DB.Password, SuperPassword: s.DB.SuperPassword, Local: local}
		u := url.URL{Scheme: "postgres", User: url.UserPassword(s.DB.User, s.DB.Password), Host: hostPort(s.DB.Host, s.DB.Port), Path: "/" + s.DB.Name, RawQuery: "sslmode=" + s.DB.sslMode()}
		c.Env["DATABASE_URL"] = u.String()
		c.Env["JWT_SECRET"] = s.JWTSecret
		sep := "/"
		if s.OS == "windows" {
			sep = `\`
		}
		c.Env["BACKUP_DIR"] = strings.TrimRight(s.Install.Dir, `/\`) + sep + "backups"
	} else {
		c.Env["FILE_DB_PORT"] = strconv.Itoa(s.FileDBPort)
	}
	if s.Backups.Dir != "" {
		c.Env["BACKUP_DIR"] = s.Backups.Dir
	}
	c.Env["BACKUP_INTERVAL"], c.Env["BACKUP_KEEP"] = defaultBackupEvery, strconv.Itoa(defaultBackupKeep)
	if s.Backups.Every != "" {
		c.Env["BACKUP_INTERVAL"] = s.Backups.Every
	}
	if s.Backups.Keep > 0 {
		c.Env["BACKUP_KEEP"] = strconv.Itoa(s.Backups.Keep)
	}
	for k, v := range s.Env {
		c.Env[k] = v
	}
	return c
}

func hostPort(host string, port int) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port)
}

// Masked hides every secret, for lists and the edit form (screen sharing).
func (s Shop) Masked() Shop {
	m := s
	if m.JWTSecret != "" {
		m.JWTSecret = maskedValue
	}
	if s.DB != nil {
		db := *s.DB
		db.Password, db.SuperPassword = maskedValue, maskedValue
		m.DB = &db
	}
	if s.Env != nil {
		m.Env = map[string]string{}
		for k, v := range s.Env {
			if secretKey.MatchString(strings.ToUpper(k)) {
				v = maskedValue
			}
			m.Env[k] = v
		}
	}
	return m
}
