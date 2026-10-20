package main

import (
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
