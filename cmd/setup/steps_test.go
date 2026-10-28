package main

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/shopconf"
)

// rec is a Runner that records commands and runs nothing.
type rec struct {
	cmds    []string
	answers map[string]string   // command prefix → output
	fail    map[string]error    // command prefix → error
	seq     map[string][]string // command prefix → outputs in turn (then answers)
}

func (r *rec) answer(cmd string) (string, error) {
	r.cmds = append(r.cmds, cmd)
	for p, outs := range r.seq {
		if strings.HasPrefix(cmd, p) && len(outs) > 0 {
			r.seq[p] = outs[1:]
			return outs[0], nil
		}
	}
	for p, err := range r.fail {
		if strings.HasPrefix(cmd, p) {
			return "", err
		}
	}
	for p, out := range r.answers {
		if strings.HasPrefix(cmd, p) {
			return out, nil
		}
	}
	return "", nil
}
func (r *rec) Run(name string, args ...string) (string, error) {
	return r.answer(strings.Join(append([]string{name}, args...), " "))
}
func (r *rec) Script(ps string) (string, error) { return r.answer("ps> " + ps) }
func (r *rec) all() string                      { return strings.Join(r.cmds, "\n") }

func conf(osName, mode string, local bool) shopconf.Conf {
	c := shopconf.Conf{Version: shopconf.Version, Shop: "Al Raj", Key: "al-raj", InstallID: "7C1E09A2", OS: osName, Mode: mode,
		Binary: "karots-suite", InstallDir: "/opt/karots-suite", Port: 3000, Open: "kiosk",
		Env: map[string]string{"APP_ENV": "production", "SERVER_PORT": "3000"}}
	if osName == "windows" {
		c.Binary, c.InstallDir = "karots-suite.exe", `C:\karots-suite`
	}
	if mode == "server" {
		host := "localhost"
		if !local {
			host = "192.168.1.20"
		}
		c.Env["BACKUP_DIR"] = c.InstallDir + map[bool]string{true: `\backups`, false: "/backups"}[osName == "windows"]
		c.DB = &shopconf.DB{Host: host, Port: 5432, Name: "suite_db", User: "suite_user", Password: "pw123", SuperPassword: "super123", Local: local}
	}
	return c
}

func ids(steps []Step) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.ID)
	}
	return out
}

func TestStepsPerRole(t *testing.T) {
	all := []string{"checks", "database", "program", "settings", "terms", "backups", "startup", "network", "printing", "open", "health"}
	for name, tc := range map[string]struct {
		c    shopconf.Conf
		role Role
		want []string
	}{
		"server local":  {conf("linux", "server", true), RoleServer, all},
		"server remote": {conf("linux", "server", false), RoleServer, all},
		"file":          {conf("linux", "file", false), RoleServer, []string{"checks", "program", "settings", "terms", "startup", "network", "printing", "open", "health"}},
		"cashier":       {conf("linux", "server", true), RoleCashier, []string{"checks", "open", "health"}},
		"windows file":  {conf("windows", "file", false), RoleServer, []string{"checks", "program", "settings", "terms", "startup", "network", "open", "health"}},
	} {
		j := &Job{Conf: tc.c, Role: tc.role}
		if got := ids(Steps(j)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
		for _, s := range Steps(j) {
			if s.Title == "" || s.Detail(j) == "" {
				t.Errorf("%s: step %s has no title / detail", name, s.ID)
			}
		}
	}
}

func TestLoadJobMissingConf(t *testing.T) {
	if _, err := loadConf(t.TempDir()); err == nil || !strings.Contains(err.Error(), "build this shop in the Shop Builder") {
		t.Fatalf("err %v", err)
	}
}

func checkJob(t *testing.T, port int) *Job {
	t.Helper()
	dir := t.TempDir()
	c := conf("linux", "file", false)
	c.Port = port
	c.InstallDir = filepath.Join(t.TempDir(), "karots-suite")
	os.WriteFile(filepath.Join(dir, c.Binary), []byte("bin"), 0o755)
	return &Job{Conf: c, Role: RoleServer, Dir: dir, User: "shop", Run: &rec{}, Log: io.Discard}
}

func TestChecksPortTaken(t *testing.T) {
	isAdminFn = func() bool { return true }
	hasSystemdFn = func() bool { return true }
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	j := checkJob(t, port)
	if err := checks(j); err == nil || !strings.Contains(err.Error(), "Shop Builder") || !strings.Contains(err.Error(), "port") {
		t.Fatalf("taken port: %v", err)
	}
	j.Updating = true // our own running install holds it
	if err := checks(j); err != nil {
		t.Fatalf("update over our own install: %v", err)
	}
	free := checkJob(t, 0)
	free.Conf.Port = freePort(t)
	if err := checks(free); err != nil {
		t.Fatalf("free port: %v", err)
	}
}

func TestChecksRefuses(t *testing.T) {
	isAdminFn = func() bool { return false }
	hasSystemdFn = func() bool { return true }
	j := checkJob(t, 0)
	j.Conf.Port = freePort(t)
	if err := checks(j); err == nil || !strings.Contains(err.Error(), "admin") {
		t.Fatalf("not admin: %v", err)
	}
	isAdminFn = func() bool { return true }
	os.Remove(filepath.Join(j.Dir, j.Conf.Binary))
	if err := checks(j); err == nil || !strings.Contains(err.Error(), j.Conf.Binary) {
		t.Fatalf("no binary: %v", err)
	}
	j = checkJob(t, 0)
	j.Conf.Port = freePort(t)
	os.MkdirAll(j.Conf.InstallDir, 0o755)
	os.WriteFile(filepath.Join(j.Conf.InstallDir, "karots-pos"), nil, 0o755)
	if err := checks(j); err == nil || !strings.Contains(err.Error(), "karots-pos") {
		t.Fatalf("over karots-pos: %v", err)
	}
}

func freePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestRunAllStops(t *testing.T) {
	var ran, marked []string
	steps := []Step{
		{ID: "a", Do: func(*Job) error { ran = append(ran, "a"); return nil }},
		{ID: "b", Do: func(*Job) error { ran = append(ran, "b"); return errors.New("boom") }},
		{ID: "c", Do: func(*Job) error { ran = append(ran, "c"); return nil }},
	}
	err := runSteps(&Job{Log: io.Discard}, steps, func(id string, err error) {
		marked = append(marked, id)
		if id == "b" && err == nil {
			t.Error("b marked ok")
		}
	})
	if err == nil || !slices.Equal(ran, []string{"a", "b"}) || !slices.Equal(marked, []string{"a", "b"}) {
		t.Fatalf("err %v ran %v marked %v", err, ran, marked)
	}
}

func TestChecksNeedShopUser(t *testing.T) {
	isAdminFn = func() bool { return true }
	hasSystemdFn = func() bool { return true }
	j := checkJob(t, 0)
	j.Conf.Port = freePort(t)
	j.User = ""
	if err := checks(j); err == nil || !strings.Contains(err.Error(), "normal user") {
		t.Fatalf("err %v", err)
	}
}

// What the person picks on the page overrides the Builder's defaults.
func TestApplyChoice(t *testing.T) {
	j, err := applyChoice(conf("linux", "file", false), "", Choice{Open: "background", BackupDir: "/media/usb/karots"})
	if err != nil || j.Conf.Open != "background" || j.Conf.Env["BACKUP_DIR"] != "/media/usb/karots" || j.Role != RoleServer {
		t.Fatalf("%+v %v", j, err)
	}
	if !slices.Contains(ids(Steps(j)), "backups") {
		t.Fatal("a chosen backup folder isn't made")
	}
	if _, err := applyChoice(conf("windows", "file", false), "", Choice{BackupDir: `backups`}); err == nil {
		t.Fatal("a relative backup folder was accepted")
	}
	if j, err := applyChoice(conf("windows", "file", false), "", Choice{BackupDir: `D:\Karots backups`}); err != nil || j.backupDir() != `D:\Karots backups` {
		t.Fatalf("windows folder: %v", err)
	}
	if _, err := applyChoice(conf("linux", "file", false), "", Choice{Open: "tv"}); err == nil {
		t.Fatal("open tv accepted")
	}
	base := conf("linux", "file", false)
	applyChoice(base, "", Choice{BackupDir: "/x"})
	if base.Env["BACKUP_DIR"] != "" {
		t.Fatal("applyChoice changed the shared install.conf env")
	}
}
