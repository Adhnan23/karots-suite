package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func hexOf(n int) *regexp.Regexp { return regexp.MustCompile(fmt.Sprintf("^[0-9a-f]{%d}$", n)) }

func mustShop(t *testing.T, key, osName, mode string) Shop {
	t.Helper()
	s, err := NewShop(key, "Al Raj", osName, "amd64", mode)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewShopServer(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	switch {
	case len(s.InstallID) != 8, !hexOf(32).MatchString(s.Channel), !hexOf(48).MatchString(s.JWTSecret):
		t.Fatalf("ids / secrets: %+v", s)
	case s.DB == nil || s.DB.Host != "localhost" || s.DB.Port != 5432 || s.DB.Name != "suite_db" || s.DB.User != "suite_user":
		t.Fatalf("db: %+v", s.DB)
	case !hexOf(32).MatchString(s.DB.Password) || !hexOf(32).MatchString(s.DB.SuperPassword):
		t.Fatalf("db passwords: %+v", s.DB)
	case s.Port != 3000 || s.Install.Dir != "/opt/karots-suite" || s.Install.Open != "kiosk":
		t.Fatalf("defaults: %+v", s)
	}
	if w := mustShop(t, "w", "windows", "server"); w.Install.Dir != `C:\karots-suite` {
		t.Fatalf("windows dir %q", w.Install.Dir)
	}
}

func TestNewShopFile(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "file")
	if s.DB != nil || s.JWTSecret != "" || s.FileDBPort != 5434 {
		t.Fatalf("%+v", s)
	}
}

func TestConfEnvServer(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	s.DB.Password = "p@ss:/w"
	s.Env = map[string]string{"JWT_EXPIRES_IN": "10h"}
	c := s.Conf("karots-suite")
	want := map[string]string{
		"APP_ENV":         "production",
		"SERVER_PORT":     "3000",
		"DATABASE_URL":    "postgres://suite_user:p%40ss%3A%2Fw@localhost:5432/suite_db?sslmode=disable",
		"JWT_SECRET":      s.JWTSecret,
		"BACKUP_DIR":      "/opt/karots-suite/backups",
		"BACKUP_INTERVAL": "6h",
		"BACKUP_KEEP":     "28",
		"JWT_EXPIRES_IN":  "10h",
	}
	if len(c.Env) != len(want) {
		t.Fatalf("env %v", c.Env)
	}
	for k, v := range want {
		if c.Env[k] != v {
			t.Errorf("%s = %q, want %q", k, c.Env[k], v)
		}
	}
	if c.DB == nil || !c.DB.Local || c.InstallID != s.InstallID || c.Binary != "karots-suite" || c.Shop != "Al Raj" {
		t.Fatalf("conf %+v", c)
	}
	w := mustShop(t, "w", "windows", "server")
	if got := w.Conf("karots-suite.exe").Env["BACKUP_DIR"]; got != `C:\karots-suite\backups` {
		t.Fatalf("windows BACKUP_DIR %q", got)
	}
	w.DB.Host = "192.168.1.20"
	if w.Conf("x").DB.Local {
		t.Fatal("a LAN database host is not local")
	}
}

func TestConfEnvFile(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "file")
	s.Env = map[string]string{"X": "1"}
	c := s.Conf("karots-suite")
	if c.DB != nil || len(c.Env) != 6 || c.Env["BACKUP_DIR"] != "" || c.Env["BACKUP_KEEP"] != "28" || c.Env["FILE_DB_PORT"] != "5434" || c.Env["SERVER_PORT"] != "3000" || c.Env["APP_ENV"] != "production" {
		t.Fatalf("env %v", c.Env)
	}
}

func TestCheckRefuses(t *testing.T) {
	all := []manifest{{Key: "alpha"}}
	for name, edit := range map[string]func(*Shop){
		"support secret": func(s *Shop) { s.Env = map[string]string{"POS_SUPPORT_SECRET": "x"} },
		"owned key":      func(s *Shop) { s.Env = map[string]string{"DATABASE_URL": "x"} },
		"plugin":         func(s *Shop) { s.Plugins = []string{"gamma"} },
		"key":            func(s *Shop) { s.Key = "Al Raj" },
		"os":             func(s *Shop) { s.OS = "darwin" },
		"port":           func(s *Shop) { s.Port = 70000 },
	} {
		s := mustShop(t, "al-raj", "linux", "server")
		edit(&s)
		if _, err := s.Check(all); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	ok := mustShop(t, "al-raj", "linux", "server")
	ok.Plugins = []string{"alpha"}
	if w, err := ok.Check(all); err != nil || len(w) != 0 {
		t.Fatalf("good shop: %v %v", w, err)
	}
}

func TestCheckWarns(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "file")
	s.Port = s.FileDBPort
	if w, err := s.Check(nil); err != nil || len(w) != 1 {
		t.Fatalf("warnings %v (%v)", w, err)
	}
}

func TestSetMode(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	if err := s.SetMode("file"); err != nil || s.DB != nil || s.JWTSecret != "" || s.FileDBPort != 5434 {
		t.Fatalf("to file: %+v %v", s, err)
	}
	if err := s.SetMode("server"); err != nil || s.DB == nil || len(s.JWTSecret) != 48 {
		t.Fatalf("to server: %+v %v", s, err)
	}
	if err := s.SetMode("cloud"); err == nil {
		t.Fatal("bad mode accepted")
	}
}

func TestRotate(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	old := s
	oldDB := *s.DB
	for _, what := range []string{"jwt", "db", "dbsuper"} {
		if err := s.Rotate(what); err != nil {
			t.Fatal(err)
		}
	}
	if s.JWTSecret == old.JWTSecret || len(s.JWTSecret) != 48 || s.DB.Password == oldDB.Password || s.DB.SuperPassword == oldDB.SuperPassword {
		t.Fatalf("not rotated: %+v", s)
	}
	if s.Rotate("channel") == nil {
		t.Fatal("unknown secret rotated")
	}
}

func TestMasked(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	s.Env = map[string]string{"AI_API_KEY": "sk-1", "JWT_EXPIRES_IN": "10h"}
	m := s.Masked()
	raw, _ := json.Marshal(m)
	for _, secret := range []string{s.JWTSecret, s.DB.Password, s.DB.SuperPassword, "sk-1"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("masked shop leaks %q", secret)
		}
	}
	if m.Env["JWT_EXPIRES_IN"] != "10h" || s.DB.Password == maskedValue || s.Env["AI_API_KEY"] != "sk-1" {
		t.Fatalf("masking changed the wrong thing: %+v / %+v", m, s)
	}
}

func TestSaveShops(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shops.json")
	if got, err := LoadShops(p); err != nil || len(got.Shops) != 0 {
		t.Fatalf("missing file: %v %v", got, err)
	}
	in := Shops{Shops: []Shop{mustShop(t, "zed", "linux", "file"), mustShop(t, "abc", "linux", "server")}}
	if err := SaveShops(p, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadShops(p)
	if err != nil || len(got.Shops) != 2 || got.Shops[0].Key != "abc" {
		t.Fatalf("load: %+v %v", got, err)
	}
}

// A failed save never leaves a half-written file in place of every shop.
func TestSaveShopsAtomic(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("needs a read-only directory")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "shops.json")
	if err := SaveShops(p, Shops{Shops: []Shop{mustShop(t, "abc", "linux", "file")}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	os.Chmod(dir, 0o500)
	defer os.Chmod(dir, 0o700)
	if err := SaveShops(p, Shops{}); err == nil {
		t.Fatal("save into a read-only folder succeeded")
	}
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Fatal("old shops.json changed by a failed save")
	}
}

func TestBackupsProfile(t *testing.T) {
	s := mustShop(t, "al-raj", "windows", "file")
	s.Backups = ShopBackups{Dir: `D:\Karots backups`, Every: "3h", Keep: 56}
	c := s.Conf("karots-suite.exe")
	if c.Env["BACKUP_DIR"] != `D:\Karots backups` || c.Env["BACKUP_INTERVAL"] != "3h" || c.Env["BACKUP_KEEP"] != "56" {
		t.Fatalf("env %v", c.Env)
	}
	if _, err := s.Check(nil); err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string]ShopBackups{
		"relative dir": {Dir: "backups"},
		"too often":    {Every: "5m"},
		"not a time":   {Every: "daily"},
		"keep":         {Keep: -1},
	} {
		x := s
		x.Backups = b
		if _, err := x.Check(nil); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	x := s
	x.Env = map[string]string{"BACKUP_KEEP": "3"}
	if _, err := x.Check(nil); err == nil {
		t.Error("BACKUP_KEEP accepted in More settings")
	}
}

// SSL to the database: off for this PC, required for anything else unless chosen.
func TestDBSSL(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "server")
	if u := s.Conf("x").Env["DATABASE_URL"]; !strings.HasSuffix(u, "sslmode=disable") {
		t.Fatalf("localhost: %s", u)
	}
	s.DB.Host = "ep-cool-name-123.ap-southeast-1.aws.neon.tech"
	if u := s.Conf("x").Env["DATABASE_URL"]; !strings.HasSuffix(u, "sslmode=require") {
		t.Fatalf("remote default: %s", u)
	}
	s.DB.SSL = "verify-full"
	if u := s.Conf("x").Env["DATABASE_URL"]; !strings.HasSuffix(u, "sslmode=verify-full") {
		t.Fatalf("chosen: %s", u)
	}
	s.DB.SSL = "disable"
	if u := s.Conf("x").Env["DATABASE_URL"]; !strings.HasSuffix(u, "sslmode=disable") {
		t.Fatalf("a LAN server without SSL: %s", u)
	}
	s.DB.SSL = "maybe"
	if _, err := s.Check(nil); err == nil {
		t.Fatal("sslmode maybe accepted")
	}
}

// Server → One PC → Server keeps the database passwords: PostgreSQL on the
// shop's PC still has the old postgres password, so a new one locked Setup out.
func TestSetModeKeepsServerPasswords(t *testing.T) {
	s := mustShop(t, "al-raj", "windows", "server")
	want := *s.DB
	if err := s.SetMode("file"); err != nil || s.DB != nil {
		t.Fatalf("to file: %+v %v", s.DB, err)
	}
	if c := s.Conf("karots-suite.exe"); c.DB != nil || c.Env["DATABASE_URL"] != "" {
		t.Fatalf("a file-mode install.conf carries the server database: %+v", c.DB)
	}
	if m := s.Masked(); strings.Contains(fmt.Sprint(m), want.SuperPassword) {
		t.Fatal("parked password shown on the page")
	}
	if err := s.SetMode("server"); err != nil || s.DB == nil || *s.DB != want {
		t.Fatalf("to server: %+v, want %+v", s.DB, want)
	}
}
