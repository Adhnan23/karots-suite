package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linuxJob runs against a temp root with apt "installed" and a fake home.
func linuxJob(t *testing.T, mode string, local bool) (*Job, *rec, string) {
	t.Helper()
	r := t.TempDir()
	root = r
	home := "/home/shop"
	homeFn = func(string) (string, error) { return home, nil }
	lookPathFn = func(name string) (string, error) {
		if name == "apt-get" || name == "chromium" {
			return "/usr/bin/" + name, nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { root = "" })
	dir := t.TempDir()
	c := conf("linux", mode, local)
	os.WriteFile(filepath.Join(dir, c.Binary), []byte("BIN"), 0o755)
	hba := filepath.Join(r, "etc", "pg_hba.conf")
	os.MkdirAll(filepath.Dir(hba), 0o755)
	os.WriteFile(hba, []byte("local all all peer\n"), 0o600)
	rc := &rec{answers: map[string]string{
		"runuser -u postgres -- psql -tAc SELECT 'up'":             "up\n",
		"runuser -u postgres -- psql -tAc SHOW port":               "5432\n",
		"runuser -u postgres -- psql -tAc SHOW server_version_num": "170005\n",
		"runuser -u postgres -- psql -p 5432 -tAc SHOW hba_file":   "/etc/pg_hba.conf\n",
	}}
	return &Job{Conf: c, Role: RoleServer, Dir: dir, User: "shop", Run: rc, Log: io.Discard}, rc, r
}

func TestLinuxServerLocal(t *testing.T) {
	j, rc, r := linuxJob(t, "server", true)
	var log strings.Builder
	j.Log = &log
	for _, s := range Steps(j) {
		if s.ID == "checks" || s.ID == "health" {
			continue
		}
		if err := s.Do(j); err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
	}
	all := rc.all()
	for _, want := range []string{
		"/usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y",
		"apt-get install -y -qq postgresql-17",
		"CREATE ROLE \"suite_user\" WITH LOGIN PASSWORD 'pw123'",
		"CREATE DATABASE \"suite_db\" OWNER \"suite_user\"",
		"systemctl enable --now postgresql",
		"systemctl daemon-reload",
		"systemctl enable karots-suite",
		"systemctl restart karots-suite",
		"apt-get install -y -qq cups",
		"chown -R shop:shop /opt/karots-suite",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(log.String(), "pw123") {
		t.Fatal("the log shows the database password")
	}
	env, err := os.ReadFile(filepath.Join(r, "opt/karots-suite/.env"))
	if err != nil || string(env) != string(j.Conf.EnvFile()) {
		t.Fatalf(".env %q (%v)", env, err)
	}
	if fi, _ := os.Stat(filepath.Join(r, "opt/karots-suite/.env")); fi.Mode().Perm() != 0o600 {
		t.Fatalf(".env mode %v", fi.Mode().Perm())
	}
	if b, _ := os.ReadFile(filepath.Join(r, "opt/karots-suite/karots-suite")); string(b) != "BIN" {
		t.Fatal("binary not copied")
	}
	unit, _ := os.ReadFile(filepath.Join(r, "etc/systemd/system/karots-suite.service"))
	if !strings.Contains(string(unit), "After=network-online.target postgresql.service") || !strings.Contains(string(unit), "User=shop") {
		t.Fatalf("unit:\n%s", unit)
	}
	hba, _ := os.ReadFile(filepath.Join(r, "etc/pg_hba.conf"))
	if !strings.HasPrefix(string(hba), "# karots-suite install") || !strings.Contains(string(hba), "local all all peer") {
		t.Fatalf("pg_hba:\n%s", hba)
	}
	if _, err := os.Stat(filepath.Join(r, "opt/karots-suite/backups")); err != nil {
		t.Fatal("no backups folder")
	}
	if _, err := os.Stat(filepath.Join(r, "opt/karots-suite/kiosk.sh")); err != nil {
		t.Fatal("no kiosk.sh")
	}
}

// A role that exists gets the profile's password, never a new one.
func TestLinuxRoleUpdatedAndPort(t *testing.T) {
	j, rc, _ := linuxJob(t, "server", true)
	j.Conf.DB.Port = 5440
	rc.answers["runuser -u postgres -- psql -p 5440 -tAc SELECT 'up'"] = "up\n"
	rc.answers["runuser -u postgres -- psql -p 5440 -tAc SELECT 1 FROM pg_roles"] = "1\n"
	rc.answers["runuser -u postgres -- psql -p 5440 -tAc SHOW hba_file"] = "/etc/pg_hba.conf\n"
	if err := linuxDatabase(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	for _, want := range []string{"ALTER SYSTEM SET port = 5440", "systemctl restart postgresql", "ALTER ROLE \"suite_user\" WITH LOGIN PASSWORD 'pw123'"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
}

func TestLinuxFileModeNoPostgres(t *testing.T) {
	j, rc, r := linuxJob(t, "file", false)
	for _, s := range Steps(j) {
		if s.ID == "checks" || s.ID == "health" {
			continue
		}
		if err := s.Do(j); err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
	}
	if strings.Contains(rc.all(), "postgres") {
		t.Fatalf("file mode touched PostgreSQL:\n%s", rc.all())
	}
	unit, _ := os.ReadFile(filepath.Join(r, "etc/systemd/system/karots-suite.service"))
	if strings.Contains(string(unit), "postgresql") {
		t.Fatalf("unit depends on postgresql:\n%s", unit)
	}
}

func TestLinuxUpdateStopsFirst(t *testing.T) {
	j, rc, _ := linuxJob(t, "file", false)
	j.Updating = true
	if err := linuxProgram(j); err != nil {
		t.Fatal(err)
	}
	if len(rc.cmds) == 0 || rc.cmds[0] != "systemctl stop karots-suite" {
		t.Fatalf("first command %v", rc.cmds)
	}
}

func TestLinuxBackground(t *testing.T) {
	j, _, r := linuxJob(t, "file", false)
	j.Conf.Open = "background"
	if err := linuxOpen(j); err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(filepath.Join(r, "home/shop/Desktop/karots-suite.desktop"))
	if err != nil || !strings.Contains(string(d), "xdg-open http://localhost:3000") {
		t.Fatalf("desktop shortcut %q (%v)", d, err)
	}
	if _, err := os.Stat(filepath.Join(r, "home/shop/.local/share/applications/karots-suite.desktop")); err != nil {
		t.Fatal("no app-menu entry")
	}
	if _, err := os.Stat(filepath.Join(r, "opt/karots-suite/kiosk.sh")); err == nil {
		t.Fatal("background made a kiosk")
	}
}

func TestLinuxCashierKiosk(t *testing.T) {
	j, _, r := linuxJob(t, "server", true)
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	if err := linuxOpen(j); err != nil {
		t.Fatal(err)
	}
	k, _ := os.ReadFile(filepath.Join(r, "opt/karots-suite/kiosk.sh"))
	if !strings.Contains(string(k), `URL="http://192.168.1.10:3000"`) {
		t.Fatalf("kiosk.sh:\n%s", k)
	}
	if _, err := os.Stat(filepath.Join(r, "home/shop/.config/autostart/karots-kiosk.desktop")); err != nil {
		t.Fatal("no autostart")
	}
}

func TestLinuxRemoteDB(t *testing.T) {
	j, rc, _ := linuxJob(t, "server", false)
	j.Conf.DB.Host, j.Conf.DB.Port = "127.0.0.1", freePort(t) // nothing listens there
	if err := linuxDatabase(j); err == nil || !strings.Contains(err.Error(), "doesn't answer") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(rc.all(), "apt-get") {
		t.Fatal("remote DB installed PostgreSQL here")
	}
}

// The suite's schema needs PostgreSQL 17: an older server is refused in words.
func TestLinuxOldPostgres(t *testing.T) {
	j, rc, _ := linuxJob(t, "server", true)
	rc.answers["runuser -u postgres -- psql -tAc SHOW server_version_num"] = "160004\n"
	if err := linuxDatabase(j); err == nil || !strings.Contains(err.Error(), "PostgreSQL 16") || !strings.Contains(err.Error(), "17") {
		t.Fatalf("err %v", err)
	}
}

// An Update on an offline shop PC: PostgreSQL 17 and CUPS already there → no package installs.
func TestLinuxUpdateOfflineNoInstalls(t *testing.T) {
	j, rc, _ := linuxJob(t, "server", true)
	lookPathFn = func(name string) (string, error) {
		if name == "apt-get" || name == "cupsd" {
			return "/usr/bin/" + name, nil
		}
		return "", os.ErrNotExist
	}
	rc.fail = map[string]error{"apt-get": os.ErrDeadlineExceeded, "/usr/share/postgresql-common": os.ErrDeadlineExceeded}
	rc.answers["runuser -u postgres -- psql -p 5432 -tAc SELECT 'up'"] = "up\n"
	rc.answers["runuser -u postgres -- psql -p 5432 -tAc SHOW server_version_num"] = "170005\n"
	if err := linuxDatabase(j); err != nil {
		t.Fatalf("database: %v", err)
	}
	if err := linuxPrinting(j); err != nil {
		t.Fatalf("printing: %v", err)
	}
	if strings.Contains(rc.all(), "apt-get") || strings.Contains(rc.all(), "pgdg") {
		t.Fatalf("installed packages again:\n%s", rc.all())
	}
}

// The user's default browser (xdg-settings) runs the kiosk when its family is known.
func TestLinuxKioskDefaultBrowser(t *testing.T) {
	j, rc, r := linuxJob(t, "file", false)
	rc.answers["runuser -u shop -- xdg-settings get default-web-browser"] = "firefox.desktop\n"
	os.MkdirAll(filepath.Join(r, "usr/share/applications"), 0o755)
	os.WriteFile(filepath.Join(r, "usr/share/applications/firefox.desktop"), []byte("[Desktop Entry]\nName=Firefox\nExec=firefox %u\n"), 0o644)
	if err := linuxOpen(j); err != nil {
		t.Fatal(err)
	}
	k, _ := os.ReadFile(filepath.Join(r, "opt/karots-suite/kiosk.sh"))
	if !strings.Contains(string(k), `firefox --kiosk -no-remote -profile "$HOME/.karots-kiosk" $URL`) || strings.Contains(string(k), "%u") {
		t.Fatalf("kiosk.sh:\n%s", k)
	}
	if _, err := os.Stat(filepath.Join(r, "etc/chromium/policies/managed/karots-kiosk.json")); err == nil {
		t.Fatal("Chromium policy written for a Firefox kiosk")
	}

	// An unknown default → a browser already installed (chromium here).
	j, rc, r = linuxJob(t, "file", false)
	rc.answers["runuser -u shop -- xdg-settings get default-web-browser"] = "someweird.desktop\n"
	if err := linuxOpen(j); err != nil {
		t.Fatal(err)
	}
	k, _ = os.ReadFile(filepath.Join(r, "opt/karots-suite/kiosk.sh"))
	if !strings.Contains(string(k), `"/usr/bin/chromium" --kiosk --app=$URL`) {
		t.Fatalf("fallback kiosk.sh:\n%s", k)
	}
}
