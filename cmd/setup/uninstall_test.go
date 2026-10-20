package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const finalCmd = "runuser -u shop -- /opt/karots-suite/karots-suite -backup /home/shop/Karots backups -tag final"

// linuxInstalled lays out an installed Linux shop under root.
func linuxInstalled(t *testing.T, r string) {
	for p, body := range map[string]string{
		"opt/karots-suite/karots-suite":                            "BIN",
		"opt/karots-suite/kiosk.sh":                                "#!/bin/sh",
		"opt/karots-suite/data/backups/al-raj-auto.json.gz":        "x",
		"etc/systemd/system/karots-suite.service":                  "[Unit]",
		"home/shop/.config/autostart/karots-kiosk.desktop":         "[Desktop Entry]",
		"etc/opt/chrome/policies/managed/karots-kiosk.json":        "{}",
		"home/shop/.local/share/applications/karots-suite.desktop": "[Desktop Entry]",
	} {
		os.MkdirAll(filepath.Dir(filepath.Join(r, p)), 0o755)
		os.WriteFile(filepath.Join(r, p), []byte(body), 0o644)
	}
}

func runUninstall(t *testing.T, j *Job) error {
	t.Helper()
	j.Uninstall = true
	return runSteps(j, Steps(j), func(string, error) {})
}

func exists(r, p string) bool { _, err := os.Stat(filepath.Join(r, p)); return err == nil }

func TestUninstallLinuxFileMode(t *testing.T) {
	j, rc, r := linuxJob(t, "file", false)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/al-raj-final.json.gz\n"
	if got := strings.Join(ids(func() []Step { j.Uninstall = true; return Steps(j) }()), " "); got != "final-backup remove-startup remove-open remove-network remove-folder done" {
		t.Fatalf("steps %s", got)
	}
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"opt/karots-suite", "etc/systemd/system/karots-suite.service", "home/shop/.config/autostart/karots-kiosk.desktop",
		"etc/opt/chrome/policies/managed/karots-kiosk.json", "home/shop/.local/share/applications/karots-suite.desktop"} {
		if exists(r, gone) {
			t.Errorf("%s still there", gone)
		}
	}
	if !exists(r, "home/shop/Karots backups/automatic/al-raj-auto.json.gz") {
		t.Error("automatic backups not kept")
	}
	if indexOf(rc.cmds, "systemctl stop karots-suite") > indexOf(rc.cmds, finalCmd) || indexOf(rc.cmds, "systemctl disable") < indexOf(rc.cmds, finalCmd) {
		t.Fatalf("order:\n%s", rc.all())
	}
	if j.FinalBackup != "/home/shop/Karots backups/al-raj-final.json.gz" {
		t.Fatalf("final backup %q", j.FinalBackup)
	}
}

// Review Focus 1: no backup → nothing removed.
func TestUninstallBackupFailsKeepsEverything(t *testing.T) {
	j, rc, r := linuxJob(t, "file", false)
	linuxInstalled(t, r)
	rc.fail = map[string]error{finalCmd: errors.New("exit status 1")}
	if err := runUninstall(t, j); err == nil || !strings.Contains(err.Error(), "nothing was removed") {
		t.Fatalf("err %v", err)
	}
	if !exists(r, "opt/karots-suite/karots-suite") || strings.Contains(rc.all(), "systemctl disable") {
		t.Fatalf("removed after a failed backup:\n%s", rc.all())
	}
}

// Review Focus 3: whatever is already gone doesn't stop the rest.
func TestUninstallHalfInstalled(t *testing.T) {
	j, rc, r := linuxJob(t, "server", true)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/x-final.json.gz\n"
	rc.fail = map[string]error{"systemctl disable": errors.New("unit not loaded"), "runuser -u postgres": errors.New("no server"), "pkill": errors.New("exit status 1")}
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	if exists(r, "opt/karots-suite") {
		t.Fatal("folder left")
	}
}

func TestUninstallServerRemovePostgres(t *testing.T) {
	j, rc, r := linuxJob(t, "server", true)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/x-final.json.gz\n"
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rc.all(), `DROP DATABASE IF EXISTS "suite_db" WITH (FORCE)`) || !strings.Contains(rc.all(), `DROP ROLE IF EXISTS "suite_user"`) || strings.Contains(rc.all(), "apt-get remove") {
		t.Fatalf("database:\n%s", rc.all())
	}

	j, rc, r = linuxJob(t, "server", true)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/x-final.json.gz\n"
	j.RemovePostgres = true
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rc.all(), "apt-get remove -y postgresql-17") {
		t.Fatalf("PostgreSQL kept:\n%s", rc.all())
	}

	// A database on another PC is never touched.
	j, rc, r = linuxJob(t, "server", false)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/x-final.json.gz\n"
	j.RemovePostgres = true
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rc.all(), "DROP") || strings.Contains(rc.all(), "apt-get remove") {
		t.Fatalf("remote database touched:\n%s", rc.all())
	}
}

func TestUninstallWindows(t *testing.T) {
	j, rc := windowsJob(t, "server")
	pgListening = func(int) bool { return true }
	t.Cleanup(func() { pgListening = listening })
	// Tests build Windows paths on Linux: at() of C:\x\y is one file named so.
	os.MkdirAll(filepath.Join(root, `C:\karots-suite`), 0o755)
	os.WriteFile(filepath.Join(root, `C:\karots-suite\karots-suite.exe`), []byte("BIN"), 0o755)
	rc.answers[`C:\karots-suite\karots-suite.exe -backup C:\Karots backups -tag final`] = "backup: C:\\Karots backups\\x-final.json.gz\r\n"
	j.RemovePostgres = true
	if err := runUninstall(t, j); err != nil {
		t.Fatalf("%v\n%s", err, rc.all())
	}
	for _, want := range []string{"Unregister-ScheduledTask -TaskName KarotsSuite", "Remove-NetFirewallRule -DisplayName 'Karots Suite'", "Karots Suite.url",
		`DROP DATABASE IF EXISTS "suite_db" WITH (FORCE)`, "uninstall-postgresql.exe"} {
		if !strings.Contains(rc.all(), want) {
			t.Errorf("no %q", want)
		}
	}
	if exists(root, `C:\karots-suite`) {
		t.Error("folder left")
	}
}

func TestUninstallCashier(t *testing.T) {
	j, _, _ := linuxJob(t, "server", true)
	j.Role, j.Uninstall = RoleCashier, true
	if got := strings.Join(ids(Steps(j)), " "); got != "remove-open done" {
		t.Fatalf("steps %s", got)
	}
}

// Review Focus 2: never remove anything but the program's own folder.
func TestSafeToRemove(t *testing.T) {
	j, _, r := linuxJob(t, "file", false)
	linuxInstalled(t, r)
	if err := safeToRemove(j); err != nil {
		t.Fatalf("install folder refused: %v", err)
	}
	os.MkdirAll(filepath.Join(r, "srv/empty"), 0o755)
	for _, dir := range []string{"/", "/opt", "/home/shop", "/usr", "opt/karots-suite", "/srv/empty"} {
		j.Conf.InstallDir = dir
		if err := safeToRemove(j); err == nil {
			t.Errorf("%s allowed", dir)
		}
	}
	j.Conf.InstallDir, j.KeepDir = "/opt/karots-suite", "/opt/karots-suite/keep"
	if err := safeToRemove(j); err == nil {
		t.Error("would remove the backups folder")
	}
	w, _ := windowsJob(t, "file")
	for _, dir := range []string{`C:\`, `C:\Program Files`, `C:\Users`, `C:\Windows`} {
		w.Conf.InstallDir = dir
		if err := safeToRemove(w); err == nil {
			t.Errorf("%s allowed", dir)
		}
	}
}

// I2: a drive root, a mount point or someone's home is never removed, even with
// the program in it.
func TestSafeToRemoveSystemFolders(t *testing.T) {
	j, _, r := linuxJob(t, "file", false)
	for _, dir := range []string{"/srv", "/home/other", "/media/usb", "/mnt/disk"} {
		os.MkdirAll(filepath.Join(r, dir), 0o755)
		os.WriteFile(filepath.Join(r, dir, j.Conf.Binary), []byte("BIN"), 0o755)
		j.Conf.InstallDir = dir
		if err := safeToRemove(j); err == nil {
			t.Errorf("%s allowed", dir)
		}
	}
	w, _ := windowsJob(t, "file")
	for _, dir := range []string{`D:\`, `D:`, `C:\Users\owner`, `C:\ProgramData`} {
		os.WriteFile(filepath.Join(root, joinOS(w, dir, w.Conf.Binary)), []byte("BIN"), 0o755)
		w.Conf.InstallDir = dir
		if err := safeToRemove(w); err == nil {
			t.Errorf("%s allowed", dir)
		}
	}
}

// I1: the backups folder is never a system folder (Setup chowns it to the shop
// user), and an uninstall only chowns what it wrote there.
func TestKeepDirRefusesSystemFolders(t *testing.T) {
	for _, k := range []string{"/", "/home", "/etc", "/home/other", "/media/usb"} {
		if _, err := applyChoice(conf("linux", "file", false), "", Choice{KeepDir: k}); err == nil {
			t.Errorf("%s accepted", k)
		}
	}
	if _, err := applyChoice(conf("windows", "file", false), "", Choice{KeepDir: `D:\`}); err == nil {
		t.Error(`D:\ accepted`)
	}
	if _, err := applyChoice(conf("linux", "file", false), "", Choice{KeepDir: "/media/usb/Karots backups"}); err != nil {
		t.Errorf("a USB folder refused: %v", err)
	}
	j, rc, r := linuxJob(t, "file", false)
	linuxInstalled(t, r)
	rc.answers[finalCmd] = "backup: /home/shop/Karots backups/al-raj-final.json.gz\n"
	if err := runUninstall(t, j); err != nil {
		t.Fatal(err)
	}
	if indexOf(rc.cmds, "chown -R shop:shop /home/shop/Karots backups/automatic") < 0 || slices.Contains(rc.cmds, "chown -R shop:shop /home/shop/Karots backups") {
		t.Fatalf("chown:\n%s", rc.all())
	}
}

// I3: PostgreSQL installed but stopped — log and go on (the final backup holds
// the shop), never loop on asking for a password.
func TestUninstallWindowsPostgresDown(t *testing.T) {
	j, rc := windowsJob(t, "server")
	pgListening = func(int) bool { return false }
	t.Cleanup(func() { pgListening = listening })
	delete(rc.answers, "ps> # karots:psql super")
	j.RemovePostgres = true
	if err := windowsRemoveDatabase(j); err != nil {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(rc.all(), "DROP DATABASE") || !strings.Contains(rc.all(), "uninstall-postgresql.exe") {
		t.Fatalf("%s", rc.all())
	}
}

// "Already gone" is fine: PowerShell exits 1 when its last command reported an
// error even with -ErrorAction SilentlyContinue (Remove-NetFirewallRule with no
// rule failed a real uninstall), so each removal script ends with exit 0.
func TestWindowsRemovalScriptsTolerateMissing(t *testing.T) {
	j, rc := windowsJob(t, "server")
	for _, f := range []func(*Job) error{windowsRemoveStartup, windowsRemoveOpen, windowsRemoveNetwork} {
		if err := f(j); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"remove-startup", "remove-open", "remove-firewall"} {
		i := indexOf(rc.cmds, "ps> # karots:"+name)
		if i < 0 || !strings.HasSuffix(strings.TrimSpace(rc.cmds[i]), "exit 0") {
			t.Errorf("%s doesn't end with exit 0", name)
		}
	}
}

// The same trap at install: background mode's shortcuts script ends by removing
// an old kiosk shortcut that is usually not there.
func TestWindowsBackgroundScriptToleratesNoKiosk(t *testing.T) {
	j, rc := windowsJob(t, "file")
	j.Conf.Open = "background"
	if err := windowsOpen(j); err != nil {
		t.Fatal(err)
	}
	i := indexOf(rc.cmds, "ps> # karots:shortcuts")
	if i < 0 || !strings.HasSuffix(strings.TrimSpace(rc.cmds[i]), "exit 0") {
		t.Fatalf("shortcuts script doesn't end with exit 0:\n%s", rc.all())
	}
}
