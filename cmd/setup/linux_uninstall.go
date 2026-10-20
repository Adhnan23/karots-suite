package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Each step undoes what the install made; whatever is already gone (a half
// install, a second Uninstall) is skipped, not an error.

func linuxRemoveStartup(j *Job) error {
	run(j, "systemctl", "disable", "--now", linuxService) //nolint:errcheck // no unit = nothing to stop
	os.Remove(at("/etc/systemd/system/" + linuxService + ".service"))
	_, err := run(j, "systemctl", "daemon-reload")
	return err
}

func linuxRemoveOpen(j *Job) error {
	home, err := homeFn(j.User)
	if err != nil {
		return fmt.Errorf("can't find %s's home folder: %w", j.User, err)
	}
	linuxNoKiosk(j, home)
	for _, p := range []string{
		filepath.Join(home, "Desktop", "karots-suite.desktop"),
		filepath.Join(home, ".local", "share", "applications", "karots-suite.desktop"),
		"/etc/chromium/policies/managed/karots-kiosk.json",
		"/etc/chromium-browser/policies/managed/karots-kiosk.json",
		"/etc/opt/chrome/policies/managed/karots-kiosk.json",
	} {
		os.Remove(at(p))
	}
	return os.RemoveAll(at(filepath.Join(home, ".karots-kiosk")))
}

// linuxRemoveNetwork closes the till port in whichever firewall is on.
func linuxRemoveNetwork(j *Job) error {
	port := fmt.Sprintf("%d/tcp", j.Conf.Port)
	if _, err := lookPathFn("ufw"); err == nil {
		if out, _ := j.Run.Run("ufw", "status"); strings.Contains(out, "Status: active") {
			run(j, "ufw", "delete", "allow", port) //nolint:errcheck // no rule = fine
		}
	}
	if _, err := lookPathFn("firewall-cmd"); err == nil {
		if _, err := j.Run.Run("systemctl", "is-active", "--quiet", "firewalld"); err == nil {
			run(j, "firewall-cmd", "--remove-port="+port, "--permanent") //nolint:errcheck
			run(j, "firewall-cmd", "--reload")                           //nolint:errcheck
		}
	}
	return nil
}

// linuxRemoveDatabase drops the shop's database and role; with RemovePostgres
// it also removes the packages linuxDatabase installs ("remove", not "purge":
// other databases' files on this PC stay on disk).
func linuxRemoveDatabase(j *Job) error {
	db, port := j.Conf.DB, strconv.Itoa(j.Conf.DB.Port)
	if _, err := psql(j, port, "DROP DATABASE IF EXISTS "+ident(db.Name)+" WITH (FORCE)"); err != nil {
		fmt.Fprintln(j.Log, "PostgreSQL doesn't answer — the shop's database was left as it is")
	} else {
		psql(j, port, "DROP ROLE IF EXISTS "+ident(db.User)) //nolint:errcheck // the database is what matters
	}
	if !j.RemovePostgres {
		return nil
	}
	pm, err := packageManager()
	if err != nil {
		return err
	}
	switch pm {
	case "apt-get":
		_, err = run(j, "apt-get", "remove", "-y", "postgresql-"+pgMajor, "postgresql-client-"+pgMajor, "postgresql-common", "postgresql-client-common")
	case "dnf":
		_, err = run(j, "dnf", "remove", "-y", "postgresql-server", "postgresql")
	case "zypper":
		_, err = run(j, "zypper", "--non-interactive", "remove", "postgresql-server", "postgresql")
	case "pacman":
		_, err = run(j, "pacman", "-R", "--noconfirm", "postgresql")
	}
	return err
}
