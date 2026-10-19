package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func windowsJob(t *testing.T, mode string) (*Job, *rec) {
	t.Helper()
	root = t.TempDir()
	t.Cleanup(func() { root = "" })
	dir := t.TempDir()
	c := conf("windows", mode, true)
	os.WriteFile(filepath.Join(dir, c.Binary), []byte("BIN"), 0o755)
	os.WriteFile(filepath.Join(dir, "karots-setup.exe"), []byte("SETUP"), 0o755)
	rc := &rec{answers: map[string]string{
		"ps> # karots:find-psql":    `C:\Program Files\PostgreSQL\17\bin\psql.exe`,
		"ps> # karots:psql super":   "up",
		"ps> # karots:find-browser": `FOUND=C:\Program Files\Google\Chrome\Application\chrome.exe`,
	}}
	return &Job{Conf: c, Role: RoleServer, Dir: dir, User: `SHOP\owner`, Run: rc, Log: io.Discard}, rc
}

func TestWindowsDatabase(t *testing.T) {
	j, rc := windowsJob(t, "server")
	var log strings.Builder
	j.Log = &log
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	for _, want := range []string{
		`--custom "--superpassword super123 --serverport 5432"`,
		"PostgreSQL.PostgreSQL.17",
		`CREATE ROLE "suite_user" WITH LOGIN PASSWORD ''pw123''`,
		`CREATE DATABASE "suite_db" OWNER "suite_user"`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(log.String(), "pw123") || strings.Contains(log.String(), "super123") {
		t.Fatal("the log shows a password")
	}
}

// PostgreSQL already there with an unknown password → ask on the page, then use it.
func TestWindowsNeedSuperPassword(t *testing.T) {
	j, rc := windowsJob(t, "server")
	delete(rc.answers, "ps> # karots:psql super")
	if err := windowsDatabase(j); !errors.Is(err, ErrNeedSuperPassword) {
		t.Fatalf("err %v", err)
	}
	j.SuperPassword = "typed-pw"
	rc.answers["ps> # karots:psql typed"] = "up"
	rc.cmds = nil
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rc.all(), "typed-pw") {
		t.Fatal("typed password not used")
	}
}

func TestWindowsUpdateStopsTaskFirst(t *testing.T) {
	j, rc := windowsJob(t, "file")
	j.Updating = true
	if err := windowsProgram(j); err != nil {
		t.Fatal(err)
	}
	if len(rc.cmds) == 0 || !strings.Contains(rc.cmds[0], "Stop-ScheduledTask -TaskName KarotsSuite") || !strings.Contains(rc.cmds[0], "Wait-Process") {
		t.Fatalf("first command %v", rc.cmds)
	}
	if b, _ := os.ReadFile(at(j.binPath())); string(b) != "BIN" {
		t.Fatal("binary not copied")
	}
}

func TestWindowsStartupHidden(t *testing.T) {
	j, rc := windowsJob(t, "file")
	if err := windowsStartup(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	for _, want := range []string{`C:\karots-suite\karots-start.exe`, `-hidden -log "C:\karots-suite\logs\karots.log" "C:\karots-suite\karots-suite.exe"`, "Register-ScheduledTask -TaskName KarotsSuite", `-AtLogOn -User 'SHOP\owner'`, `C:\karots-suite\karots-suite.exe`} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
}

func TestWindowsOpen(t *testing.T) {
	j, rc := windowsJob(t, "file")
	j.Conf.Open = "background"
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	if all := rc.all(); !strings.Contains(all, "Karots Suite.url") || !strings.Contains(all, "URL=http://localhost:3000") || strings.Contains(all, "kiosk.cmd") {
		t.Fatalf("background:\n%s", all)
	}
	j, rc = windowsJob(t, "file")
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	k, err := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "kiosk.cmd")))
	if err != nil || !strings.Contains(string(k), "--kiosk --app=http://localhost:3000") || !strings.Contains(string(k), `chrome.exe`) {
		t.Fatalf("kiosk.cmd %q (%v)", k, err)
	}
	if !strings.Contains(rc.all(), "CommonStartup") || !strings.Contains(rc.all(), "karots-start.exe") || strings.Contains(rc.all(), "conhost") {
		t.Fatalf("Startup shortcut must run the loop with no window:\n%s", rc.all())
	}
}

func TestWindowsSettingsAndNoPrinting(t *testing.T) {
	j, _ := windowsJob(t, "server")
	if err := windowsSettings(j); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, ".env"))); string(b) != string(j.Conf.EnvFile()) {
		t.Fatalf(".env %q", b)
	}
	for _, s := range Steps(j) {
		if s.ID == "printing" {
			t.Fatal("a printing step on Windows")
		}
	}
}

// No Chrome and no winget: winget's error text is never taken for a browser;
// Edge (on every Windows 10/11) runs the kiosk instead.
func TestWindowsKioskBrowser(t *testing.T) {
	j, rc := windowsJob(t, "file")
	rc.answers["ps> # karots:find-browser"] = "winget : The term 'winget' is not recognized\n"
	if err := windowsOpen(j); err == nil || !strings.Contains(err.Error(), "Chrome") {
		t.Fatalf("no browser: %v", err)
	}
	j, rc = windowsJob(t, "file")
	rc.answers["ps> # karots:find-browser"] = "noise\nFOUND=C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe\n"
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	k, _ := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "kiosk.cmd")))
	if !strings.Contains(string(k), `msedge.exe" --kiosk http://localhost:3000 --edge-kiosk-type=fullscreen`) || strings.Contains(string(k), "--app=") || strings.Contains(string(k), "noise") {
		t.Fatalf("kiosk.cmd:\n%s", k)
	}
	if !strings.Contains(rc.all(), `UrlAssociations\http\UserChoice`) || strings.Contains(rc.all(), "winget") || !strings.Contains(rc.all(), `Microsoft\Edge\Application\msedge.exe`) {
		t.Fatalf("find-browser script:\n%s", rc.all())
	}
}

// The PC's default browser runs the kiosk when it's Chrome-style or Firefox-style.
func TestWindowsKioskDefaultBrowser(t *testing.T) {
	for name, tc := range map[string]struct{ out, want, wantArgs string }{
		"helium":  {"DEFAULT=C:\\Users\\o\\AppData\\Local\\imput\\Helium\\Application\\helium.exe\nFOUND=C:\\Edge\\msedge.exe", `helium.exe" --kiosk --app=`, ""},
		"firefox": {"DEFAULT=C:\\Program Files\\Mozilla Firefox\\firefox.exe\nFOUND=C:\\Edge\\msedge.exe", `firefox.exe" --kiosk -no-remote -profile "C:\karots-suite\kiosk-profile" http://localhost:3000`, ""},
		"unknown": {"DEFAULT=C:\\Program Files\\Internet Explorer\\iexplore.exe\nFOUND=C:\\Edge\\msedge.exe", `msedge.exe" --kiosk http://localhost:3000`, ""},
	} {
		j, rc := windowsJob(t, "file")
		rc.answers["ps> # karots:find-browser"] = tc.out
		if err := windowsOpen(j); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		k, _ := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "kiosk.cmd")))
		if !strings.Contains(string(k), tc.want) {
			t.Errorf("%s: kiosk.cmd lacks %q:\n%s", name, tc.want, k)
		}
		if name == "firefox" && strings.Contains(string(k), "--disable-infobars") {
			t.Errorf("firefox got Chrome-only flags:\n%s", k)
		}
	}
}

// Setup copies itself (a no-window program) in as karots-start.exe, which starts
// Karots and the kiosk loop hidden — main PC and cashier PC alike.
func TestWindowsLauncherCopied(t *testing.T) {
	j, rc := windowsJob(t, "file")
	if err := windowsStartup(j); err != nil {
		t.Fatal(err)
	}
	if i, k := strings.Index(rc.all(), "stop-launcher"), strings.Index(rc.all(), "startup-task"); i < 0 || i > k {
		t.Fatalf("a running karots-start.exe must be stopped before it's replaced:\n%s", rc.all())
	}
	if b, _ := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "karots-start.exe"))); string(b) != "SETUP" {
		t.Fatal("karots-start.exe not copied for the start-up task")
	}
	j, _ = windowsJob(t, "file")
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(at(joinOS(j, j.Conf.InstallDir, "karots-start.exe"))); err != nil {
		t.Fatal("karots-start.exe not copied on a cashier PC")
	}
}
