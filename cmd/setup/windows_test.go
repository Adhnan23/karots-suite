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
	rc := &rec{answers: map[string]string{
		"ps> # karots:find-psql":    `C:\Program Files\PostgreSQL\17\bin\psql.exe`,
		"ps> # karots:psql super":   "up",
		"ps> # karots:find-browser": `BROWSER=C:\Program Files\Google\Chrome\Application\chrome.exe`,
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
	for _, want := range []string{"-WindowStyle Hidden", "Register-ScheduledTask -TaskName KarotsSuite", `-AtLogOn -User 'SHOP\owner'`, `C:\karots-suite\karots-suite.exe`} {
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
	if !strings.Contains(rc.all(), "CommonStartup") {
		t.Fatal("no Startup shortcut")
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
	rc.answers["ps> # karots:find-browser"] = "noise\nBROWSER=C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe\n"
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	k, _ := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "kiosk.cmd")))
	if !strings.Contains(string(k), `msedge.exe" --kiosk http://localhost:3000 --edge-kiosk-type=fullscreen`) || strings.Contains(string(k), "--app=") || strings.Contains(string(k), "noise") {
		t.Fatalf("kiosk.cmd:\n%s", k)
	}
	if !strings.Contains(rc.all(), "Get-Command winget") || !strings.Contains(rc.all(), `Microsoft\Edge\Application\msedge.exe`) {
		t.Fatalf("find-browser script:\n%s", rc.all())
	}
}
