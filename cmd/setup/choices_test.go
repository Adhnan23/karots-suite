package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/shopconf"
)

// SET-10: this PC's choices are checked before any step runs.
func TestSET10_ChoiceLimits(t *testing.T) {
	lin, win := conf("linux", "file", false), conf("windows", "file", false)
	for _, tc := range []struct {
		name string
		c    shopconf.Conf
		ch   Choice
		bad  string // "" = accepted; else a word the refusal must hold
	}{
		{"every 30m", lin, Choice{BackupEvery: "30m"}, ""},
		{"every 29m", lin, Choice{BackupEvery: "29m"}, "at least 30 minutes"},
		{"every junk", lin, Choice{BackupEvery: "often"}, "at least 30 minutes"},
		{"keep 1", lin, Choice{BackupKeep: 1}, ""},
		{"keep 999", lin, Choice{BackupKeep: 999}, ""},
		{"keep 1000", lin, Choice{BackupKeep: 1000}, "1 to 999"},
		{"keep -1", lin, Choice{BackupKeep: -1}, "1 to 999"},
		{"linux backups relative", lin, Choice{BackupDir: "backups"}, "/home/backups/karots"},
		{"windows backups relative", win, Choice{BackupDir: "backups"}, `D:\Karots backups`},
		{"windows backups D:/", win, Choice{BackupDir: "D:/Karots backups"}, ""},
		{"windows backups share", win, Choice{BackupDir: `\\nas\karots`}, ""},
		{"windows backups D:x", win, Choice{BackupDir: `D:x`}, `D:\Karots backups`},
		{"windows backups D:", win, Choice{BackupDir: `D:`}, `D:\Karots backups`},
		{"windows copies relative", win, Choice{CopyDir: "usb"}, `E:\Karots copies`},
		{"linux copies relative", lin, Choice{CopyDir: "usb"}, "/media/usb/Karots copies"},
		{"windows keep dir relative", win, Choice{KeepDir: `shop\kept`}, `D:\Karots backups`},
		{"linux keep dir relative", lin, Choice{KeepDir: "shop/kept"}, "/media/usb/Karots backups"},
		{"role", lin, Choice{Role: "till"}, "server | cashier"},
	} {
		j, err := applyChoice(tc.c, "", tc.ch)
		switch {
		case tc.bad == "" && err != nil:
			t.Errorf("%s: refused: %v", tc.name, err)
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%s: %v, want a refusal naming %q", tc.name, err, tc.bad)
		case tc.bad == "" && tc.ch.BackupKeep != 0 && j.Conf.Env["BACKUP_KEEP"] == "":
			t.Errorf("%s: not kept in the env", tc.name)
		}
	}
}

// SET-10: the cashier PC's main PC address — typed, else the Builder's, the port added.
func TestSET10_CashierAddress(t *testing.T) {
	c := conf("linux", "file", false)
	for typed, want := range map[string]string{
		"192.168.1.10":           "192.168.1.10:3000",
		" http://shop-pc/ ":      "shop-pc:3000",
		"https://10.0.0.2:8080/": "10.0.0.2:8080",
	} {
		j, err := applyChoice(c, "", Choice{Role: RoleCashier, Address: typed})
		if err != nil || j.Address != want {
			t.Errorf("%q: %v %q, want %q", typed, err, j.Address, want)
		}
	}
	if _, err := applyChoice(c, "", Choice{Role: RoleCashier}); err == nil || !strings.Contains(err.Error(), "-address") {
		t.Fatalf("no address anywhere: %v", err)
	}
	c.LanAddress = "192.168.1.5"
	if j, err := applyChoice(c, "", Choice{Role: RoleCashier}); err != nil || j.Address != "192.168.1.5:3000" {
		t.Fatalf("the Builder's address: %v %+v", err, j)
	}
	if j, _ := applyChoice(c, "", Choice{Role: RoleCashier, Address: "10.0.0.9"}); j.Address != "10.0.0.9:3000" {
		t.Fatalf("the typed address must win: %q", j.Address)
	}
}

// SET-10 / SET-3: an Update on a main PC keeps this PC's choices and notices a mode
// change; a cashier PC's Update takes none of the main PC's.
func TestSET10_UpdateKeepsOnlyOnTheMainPC(t *testing.T) {
	dir, inst := t.TempDir(), t.TempDir()
	c := conf("linux", "file", false)
	c.InstallDir = inst
	if err := shopconf.Write(filepath.Join(dir, shopconf.FileName), c); err != nil {
		t.Fatal(err)
	}
	j, err := jobIn(dir, Choice{})
	if err != nil || j.Updating || len(j.Kept) != 0 || j.FromMode != "" {
		t.Fatalf("first install: %v %+v", err, j)
	}

	os.WriteFile(filepath.Join(inst, c.Binary), []byte("BIN"), 0o755)
	os.WriteFile(filepath.Join(inst, ".env"), []byte("SERVER_PORT=3000\nBACKUP_DIR=/media/usb/kept\n"), 0o600)
	os.Mkdir(filepath.Join(inst, "data"), 0o755)
	j, err = jobIn(dir, Choice{})
	if err != nil || !j.Updating || !slices.Contains(j.Kept, "BACKUP_DIR") || j.backupDir() != "/media/usb/kept" || j.FromMode != "" {
		t.Fatalf("main PC Update, same mode: %v %+v", err, j)
	}

	os.WriteFile(filepath.Join(inst, ".env"), []byte("DATABASE_URL=postgres://x\n"), 0o600)
	if j, _ = jobIn(dir, Choice{}); j.FromMode != "server" {
		t.Fatalf("main PC Update from server mode: %q", j.FromMode)
	}

	j, err = jobIn(dir, Choice{Role: RoleCashier, Address: "192.168.1.10"})
	if err != nil || !j.Updating || len(j.Kept) != 0 || j.FromMode != "" {
		t.Fatalf("cashier PC Update: %v kept %v from %q", err, j.Kept, j.FromMode)
	}

	if _, err := jobIn(t.TempDir(), Choice{}); err == nil || !strings.Contains(err.Error(), shopconf.FileName) {
		t.Fatalf("no install.conf: %v", err)
	}
}

// SET-10: the shop user is whoever started Setup, never root.
func TestSET10_ShopUser(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"KAROTS_SETUP_USER": "ali", "SUDO_USER": "sam", "USER": "root"}, "ali"},
		{map[string]string{"KAROTS_SETUP_USER": "root", "SUDO_USER": "sam", "USER": "root"}, "sam"},
		{map[string]string{"SUDO_USER": "root", "USER": "root"}, ""},
		{map[string]string{"USER": "owner"}, "owner"},
		{map[string]string{"USERDOMAIN": "SHOP", "USERNAME": "owner", "USER": "x"}, `SHOP\owner`},
	} {
		for _, k := range []string{"KAROTS_SETUP_USER", "SUDO_USER", "USERDOMAIN", "USERNAME", "USER"} {
			t.Setenv(k, tc.env[k])
		}
		if got := shopUser(); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.env, got, tc.want)
		}
	}
	if !absFor("windows", `D:\`) || absFor("windows", "D:") || !absFor("linux", "/") {
		t.Fatal("absFor at its shortest")
	}
	if osName("") != "" || osName("windows") != "Windows" || osName("linux") != "Linux" {
		t.Fatal("osName")
	}
}
