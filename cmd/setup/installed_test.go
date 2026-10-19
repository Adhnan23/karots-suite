package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(p, []byte("# note\nA=1\n\nB = two words\r\nBAD LINE\n"), 0o600)
	env := readEnvFile(p)
	if env["A"] != "1" || env["B"] != "two words" || len(env) != 2 {
		t.Fatalf("%v", env)
	}
	if readEnvFile(filepath.Join(t.TempDir(), "missing")) != nil {
		t.Fatal("a missing file should read as nil")
	}
}

// An Update keeps this PC's own choices and hand-added lines; what the Builder
// owns (database, secret, port, its More settings) comes from install.conf.
func TestApplyInstalled(t *testing.T) {
	c := conf("linux", "server", true)
	c.Env["DATABASE_URL"] = "postgres://new"
	c.Env["JWT_EXPIRES_IN"] = "10h" // the Builder's More settings
	installed := map[string]string{
		"DATABASE_URL":    "postgres://old-hand-edit",
		"SERVER_PORT":     "3999",
		"BACKUP_DIR":      "/media/usb/karots",
		"BACKUP_INTERVAL": "3h",
		"BACKUP_KEEP":     "56",
		"JWT_EXPIRES_IN":  "1h",
		"PRINTER_HACK":    "yes", // added by hand
	}
	got, kept := applyInstalled(c, installed, "background")
	want := map[string]string{"DATABASE_URL": "postgres://new", "SERVER_PORT": "3000", "BACKUP_DIR": "/media/usb/karots",
		"BACKUP_INTERVAL": "3h", "BACKUP_KEEP": "56", "JWT_EXPIRES_IN": "10h", "PRINTER_HACK": "yes"}
	for k, v := range want {
		if got.Env[k] != v {
			t.Errorf("%s = %q, want %q", k, got.Env[k], v)
		}
	}
	if got.Open != "background" {
		t.Errorf("open %q", got.Open)
	}
	if !slices.Equal(kept, []string{"BACKUP_DIR", "BACKUP_INTERVAL", "BACKUP_KEEP", "PRINTER_HACK", "start: background"}) {
		t.Errorf("kept %v", kept)
	}
	if c.Env["BACKUP_DIR"] == "/media/usb/karots" {
		t.Error("applyInstalled changed the shared install.conf env")
	}
}

// On an Update the page starts from this PC's values, and a new pick still wins.
func TestUpdatePageChoiceWins(t *testing.T) {
	c, _ := applyInstalled(conf("linux", "file", false), map[string]string{"BACKUP_DIR": "/media/usb/karots"}, "")
	j, err := applyChoice(c, "", Choice{})
	if err != nil || j.backupDir() != "/media/usb/karots" {
		t.Fatalf("kept folder lost: %v %v", j, err)
	}
	j, _ = applyChoice(c, "", Choice{BackupDir: "/srv/backups"})
	if j.backupDir() != "/srv/backups" {
		t.Fatalf("page pick lost: %s", j.backupDir())
	}
}
