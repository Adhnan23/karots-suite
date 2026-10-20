package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgramBackupLinux(t *testing.T) {
	j, rc, r := linuxJob(t, "file", false)
	cmd := "runuser -u shop -- /opt/karots-suite/karots-suite -backup /home/shop/Karots backups -tag final"
	rc.answers[cmd] = "2026/10/20 file mode…\nbackup: /home/shop/Karots backups/al-raj-20261020-final.json.gz\n"
	got, err := programBackup(j, "final")
	if err != nil || got != "/home/shop/Karots backups/al-raj-20261020-final.json.gz" {
		t.Fatalf("%q %v", got, err)
	}
	if !strings.Contains(rc.all(), "chown shop:shop /home/shop/Karots backups") {
		t.Fatalf("folder not handed to the shop user:\n%s", rc.all())
	}
	if _, err := os.Stat(filepath.Join(r, "home/shop/Karots backups")); err != nil {
		t.Fatal("folder not made")
	}

	rc.answers[cmd] = "port 3000 is in use\n"
	if _, err := programBackup(j, "final"); err == nil || !strings.Contains(err.Error(), "port 3000 is in use") {
		t.Fatalf("no backup line → %v", err)
	}
}

func TestProgramBackupWindows(t *testing.T) {
	j, rc := windowsJob(t, "file")
	rc.answers[`C:\karots-suite\karots-suite.exe -backup C:\Karots backups -tag before-move`] = "backup: C:\\Karots backups\\x-before-move.json.gz\r\n"
	got, err := programBackup(j, "before-move")
	if err != nil || got != `C:\Karots backups\x-before-move.json.gz` {
		t.Fatalf("%q %v\n%s", got, err, rc.all())
	}
}

func TestProgramRestore(t *testing.T) {
	j, rc, _ := linuxJob(t, "server", true)
	rc.answers["runuser -u shop -- /opt/karots-suite/karots-suite -restore /media/usb/x.json.gz"] = "goose: no migrations\nrestored: 3192 rows\n"
	got, err := programRestore(j, "/media/usb/x.json.gz")
	if err != nil || got != "restored: 3192 rows" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestFindBackups(t *testing.T) {
	j, _, r := linuxJob(t, "file", false)
	keep := filepath.Join(r, "home/shop/Karots backups")
	os.MkdirAll(keep, 0o755)
	old := time.Now().Add(-48 * time.Hour)
	for name, at := range map[string]time.Time{"a-old.json.gz": old, "b-mid.json.gz": old.Add(time.Hour), "c.json.gz.tmp": time.Now()} {
		p := filepath.Join(keep, name)
		os.WriteFile(p, []byte("x"), 0o644)
		os.Chtimes(p, at, at)
	}
	os.WriteFile(filepath.Join(j.Dir, "al-raj-final.json.gz"), []byte("x"), 0o644)
	got := findBackups(j)
	if len(got) != 3 || got[0].Name != "al-raj-final.json.gz" || !got[0].BesideSetup || got[1].Name != "b-mid.json.gz" || got[2].BesideSetup {
		t.Fatalf("%+v", got)
	}
	if got[1].Path != "/home/shop/Karots backups/b-mid.json.gz" {
		t.Fatalf("path %q", got[1].Path)
	}
}

const (
	beforeMoveCmd = "runuser -u shop -- /opt/karots-suite/karots-suite -backup /home/shop/Karots backups -tag before-move"
	restoreCmd    = "runuser -u shop -- /opt/karots-suite/karots-suite -restore"
)

func indexOf(cmds []string, prefix string) int {
	for i, c := range cmds {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

func TestStepsOrderWithModeChange(t *testing.T) {
	j, _, _ := linuxJob(t, "server", true)
	j.Updating, j.FromMode = true, "file"
	got := strings.Join(ids(Steps(j)), " ")
	if got != "checks safety-backup database program settings backups restore startup network printing open health" {
		t.Fatalf("steps %s", got)
	}
}

func TestFirstInstallRestore(t *testing.T) {
	j, _, _ := linuxJob(t, "file", false)
	j.Restore = "/media/usb/x.json.gz"
	got := strings.Join(ids(Steps(j)), " ")
	if !strings.Contains(got, "settings restore startup") || strings.Contains(got, "safety-backup") {
		t.Fatalf("steps %s", got)
	}
}

// File → server: the old program backs up before anything changes; the new one
// restores once its settings are written and before it starts with the PC.
func TestModeChangeFileToServer(t *testing.T) {
	j, rc, r := linuxJob(t, "server", true)
	j.Updating, j.FromMode, j.ChangeMode = true, "file", true
	os.MkdirAll(filepath.Join(r, "opt/karots-suite/data/pg"), 0o755)
	rc.answers[beforeMoveCmd] = "backup: /home/shop/Karots backups/al-raj-before-move.json.gz\n"
	rc.answers[restoreCmd] = "restored: 5 rows\n"
	for _, s := range Steps(j) {
		if s.ID == "checks" || s.ID == "health" {
			continue
		}
		if err := s.Do(j); err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
	}
	backup, restore := indexOf(rc.cmds, beforeMoveCmd), indexOf(rc.cmds, restoreCmd+" /home/shop/Karots backups/al-raj-before-move.json.gz")
	copied, env, enable := indexOf(rc.cmds, "chown -R shop:shop /opt/karots-suite"), indexOf(rc.cmds, "chown shop:shop /opt/karots-suite/.env"), indexOf(rc.cmds, "systemctl enable karots-suite")
	if backup < 0 || backup > copied || backup > indexOf(rc.cmds, "runuser -u postgres") || restore < env || restore > enable {
		t.Fatalf("order backup %d copy %d env %d restore %d enable %d:\n%s", backup, copied, env, restore, enable, rc.all())
	}
	if indexOf(rc.cmds, "systemctl stop karots-suite") > backup {
		t.Fatal("backed up while the till was running")
	}
	if _, err := os.Stat(filepath.Join(r, "opt/karots-suite/data-before-server-"+time.Now().Format("2006-01-02"), "pg")); err != nil {
		t.Fatalf("old data not set aside: %v", err)
	}
}

// A bad backup file: the till is never started on an empty database.
func TestRestoreBadFileStopsBeforeStartup(t *testing.T) {
	j, rc, _ := linuxJob(t, "file", false)
	j.Restore = "/media/usb/bad.json.gz"
	rc.fail = map[string]error{restoreCmd: errors.New("exit status 1")}
	var steps []Step
	for _, s := range Steps(j) {
		if s.ID != "checks" && s.ID != "health" {
			steps = append(steps, s)
		}
	}
	err := runSteps(j, steps, func(string, error) {})
	if err == nil || !strings.Contains(err.Error(), "Bring back the shop") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(rc.all(), "systemctl enable karots-suite") {
		t.Fatal("started with the PC after a failed restore")
	}
}
