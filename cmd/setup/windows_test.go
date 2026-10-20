package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/shopconf"
)

func windowsJob(t *testing.T, mode string) (*Job, *rec) {
	t.Helper()
	root = t.TempDir()
	tillPortFree, stopPause = func(int) bool { return true }, 0
	t.Cleanup(func() { root, tillPortFree, stopPause = "", portFree, time.Second/2 })
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

const psqlPath = `C:\Program Files\PostgreSQL\17\bin\psql.exe`

func TestWindowsDatabase(t *testing.T) {
	j, rc := windowsJob(t, "server")
	rc.seq = map[string][]string{"ps> # karots:find-psql": {"", psqlPath}} // not there, then installed by winget
	var log strings.Builder
	j.Log = &log
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	for _, want := range []string{
		`--custom "--superpassword super123 --serverport 5432 --disable-components pgAdmin,stackbuilder"`,
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
	if !strings.Contains(log.String(), "@@NOTE Installing PostgreSQL 17 with winget") {
		t.Fatalf("no progress note on the page:\n%s", log.String())
	}
}

// PostgreSQL already on the PC: nothing is installed or downloaded.
func TestWindowsDatabaseAlreadyThere(t *testing.T) {
	j, rc := windowsJob(t, "server")
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	if all := rc.all(); strings.Contains(all, "winget install") || strings.Contains(all, "Invoke-WebRequest") {
		t.Fatalf("installed again:\n%s", all)
	}
}

// No winget (some Windows 10 builds): the official installer winget would run is
// downloaded instead, checked against its pinned SHA-256, and run silently.
func TestWindowsDatabaseWithoutWinget(t *testing.T) {
	j, rc := windowsJob(t, "server")
	rc.seq = map[string][]string{"ps> # karots:find-psql": {"", "", psqlPath}} // not there, winget missing, then installed
	rc.answers["ps> # karots:winget-postgresql"] = "no-winget"
	var log strings.Builder
	j.Log = &log
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@@NOTE Downloading PostgreSQL 17 (413 MB)", "@@NOTE Installing PostgreSQL 17"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("no %q on the page:\n%s", want, log.String())
		}
	}
	all := rc.all()
	for _, want := range []string{
		"Get-Command winget",
		"https://get.enterprisedb.com/postgresql/postgresql-17.11-5-windows-x64.exe",
		"612C7F5400A003AAFFA0372CB273EED52278F93C680AB911EB9CB2229E329072",
		"Get-FileHash",
		"--mode unattended --unattendedmodeui none --superpassword super123 --serverport 5432 --disable-components pgAdmin,stackbuilder",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	// Not installed after all: say so without blaming the internet alone.
	j, rc = windowsJob(t, "server")
	rc.answers["ps> # karots:find-psql"] = ""
	rc.answers["ps> # karots:winget-postgresql"] = "no-winget"
	if err := windowsDatabase(j); err == nil || !strings.Contains(err.Error(), "get.enterprisedb.com") {
		t.Fatalf("err %v", err)
	}
}

// The installer the Builder put beside the three files (a USB stick, no
// internet at the shop) is used first: checked, run, nothing downloaded.
func TestWindowsDatabaseInstallerBeside(t *testing.T) {
	j, rc := windowsJob(t, "server")
	rc.seq = map[string][]string{"ps> # karots:find-psql": {"", psqlPath}}
	os.WriteFile(filepath.Join(j.Dir, shopconf.PGInstallerName), []byte("EXE"), 0o755)
	var log strings.Builder
	j.Log = &log
	if err := windowsDatabase(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	for _, want := range []string{"# karots:local-postgresql", filepath.Join(j.Dir, shopconf.PGInstallerName), shopconf.PGInstallerSHA256,
		"--mode unattended --unattendedmodeui none --superpassword super123 --serverport 5432 --disable-components pgAdmin,stackbuilder"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "winget install") || strings.Contains(all, "Invoke-WebRequest") {
		t.Fatalf("downloaded although the installer was beside Setup:\n%s", all)
	}
	if !strings.Contains(log.String(), "@@NOTE Installing PostgreSQL 17 from the copy beside Setup") {
		t.Fatalf("no note:\n%s", log.String())
	}
}

// A step that runs long says it is still alive, under its spinner.
func TestPSHeartbeat(t *testing.T) {
	defer func(d time.Duration) { heartbeat = d }(heartbeat)
	heartbeat = 20 * time.Millisecond
	j, _ := windowsJob(t, "server")
	j.Run = slowRunner{80 * time.Millisecond}
	var log syncBuilder
	j.Log = &log
	j.note("Installing PostgreSQL 17")
	ps(j, "# karots:install-postgresql\nStart-Process x -Wait")
	if !strings.Contains(log.String(), "@@NOTE Installing PostgreSQL 17 · still working") {
		t.Fatalf("no heartbeat:\n%s", log.String())
	}
}

type slowRunner struct{ d time.Duration }

func (s slowRunner) Run(string, ...string) (string, error) { time.Sleep(s.d); return "", nil }
func (s slowRunner) Script(string) (string, error)         { time.Sleep(s.d); return "", nil }

type syncBuilder struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuilder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuilder) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

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
	if all := rc.all(); !strings.Contains(all, "Karots Suite.url") || !strings.Contains(all, "URL=http://localhost:3000") || strings.Contains(all, "kiosk.cmd") ||
		!strings.Contains(all, "Unregister-ScheduledTask -TaskName 'KarotsSuiteKiosk'") || !strings.Contains(all, "Karots Suite Kiosk.lnk") {
		t.Fatalf("background (a kiosk set up before must go):\n%s", all)
	}
	j, rc = windowsJob(t, "file")
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	k, err := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, "kiosk.cmd")))
	if err != nil || !strings.Contains(string(k), "--kiosk --app=http://localhost:3000") || !strings.Contains(string(k), `chrome.exe`) ||
		!strings.Contains(string(k), ":end\r\nexit /b 0") { // a non-zero exit would make karots-start start the loop again
		t.Fatalf("kiosk.cmd %q (%v)", k, err)
	}
	// A sign-in task, not a Startup-folder shortcut: Windows holds Startup items
	// back ~10 s after sign-in. Any user who signs in gets it, not elevated.
	all := rc.all()
	for _, want := range []string{"Register-ScheduledTask -TaskName 'KarotsSuiteKiosk'", "-AtLogOn", "-GroupId 'S-1-5-32-545'",
		"-RunLevel Limited", "karots-start.exe", "-hidden cmd.exe /c", "Remove-Item", "Karots Suite Kiosk.lnk"} {
		if !strings.Contains(all, want) {
			t.Errorf("kiosk task: missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "CreateShortcut") || strings.Contains(all, "conhost") {
		t.Fatalf("no Startup shortcut, no console window:\n%s", all)
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

// One Setup run stops / copies karots-start.exe once: the open step must not
// kill the copy that the start-up step just used to start Karots.
func TestWindowsLauncherOncePerRun(t *testing.T) {
	j, rc := windowsJob(t, "file")
	if err := windowsStartup(j); err != nil {
		t.Fatal(err)
	}
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(rc.all(), "# karots:stop-launcher"); n != 1 {
		t.Fatalf("stop-launcher ran %d times:\n%s", n, rc.all())
	}
}

// An Update stops karots-start before Karots: killed Karots is a "crash" the
// launcher would restart 5 s later, locking the exe Setup is replacing.
func TestWindowsStopOrder(t *testing.T) {
	j, rc := windowsJob(t, "server")
	j.Updating = true
	if err := windowsProgram(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	launcher, karots := strings.Index(all, `karots-start.exe`), strings.Index(all, `karots-suite.exe'`)
	if launcher < 0 || karots < 0 || launcher > karots {
		t.Fatalf("karots-start must be stopped before Karots:\n%s", all)
	}
	if !strings.Contains(all, "kiosk.cmd") {
		t.Fatalf("the old kiosk loop must be stopped too:\n%s", all)
	}
}

// Windows holds a just-stopped .exe (or antivirus a just-written one) for a
// moment: writes are retried instead of failing the step.
func TestRetryWrite(t *testing.T) {
	defer func(d time.Duration) { retryPause = d }(retryPause)
	retryPause = time.Millisecond
	tries := 0
	err := retryWrite(func() error {
		tries++
		if tries < 3 {
			return errors.New("being used by another process")
		}
		return nil
	})
	if err != nil || tries != 3 {
		t.Fatalf("err %v after %d tries", err, tries)
	}
	tries = 0
	if err := retryWrite(func() error { tries++; return errors.New("locked") }); err == nil || tries != retryTries {
		t.Fatalf("gives up after %d tries: %v (%d)", retryTries, err, tries)
	}
}
