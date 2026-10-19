package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	hbaMarker = "# karots-suite install"
	// The schema uses PostgreSQL 17 settings (transaction_timeout); file mode runs 17 too.
	pgMajor      = "17"
	minPGVersion = 170000
)

func atoi(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }

// linuxDatabase makes the shop's PostgreSQL: on this PC it installs and starts
// it, sets its port, and makes the role (with the profile's password — an
// existing role gets it too) and the database. A database elsewhere is only checked.
func linuxDatabase(j *Job) error {
	db := j.Conf.DB
	if !db.Local {
		addr := net.JoinHostPort(db.Host, strconv.Itoa(db.Port))
		c, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			return fmt.Errorf("the database at %s doesn't answer — check that PC is on and its PostgreSQL allows this one", addr)
		}
		c.Close()
		fmt.Fprintln(j.Log, "the database at", addr, "answers")
		return nil
	}
	pm, err := packageManager()
	if err != nil {
		return err
	}
	switch pm {
	case "apt-get":
		// Debian / Ubuntu ship older majors; the PostgreSQL project's own apt
		// repo has 17 (its setup script comes with postgresql-common).
		if err = install(j, "postgresql-common", "ca-certificates"); err == nil {
			if _, err = run(j, "/usr/share/postgresql-common/pgdg/apt.postgresql.org.sh", "-y"); err == nil {
				err = install(j, "postgresql-"+pgMajor)
			}
		}
	case "dnf", "zypper":
		err = install(j, "postgresql-server", "postgresql")
	default:
		err = install(j, "postgresql")
	}
	if err != nil {
		return err
	}
	switch pm { // Fedora / Arch don't make a cluster on install
	case "dnf":
		if _, err := os.Stat(at("/var/lib/pgsql/data/PG_VERSION")); err != nil {
			run(j, "postgresql-setup", "--initdb") //nolint:errcheck // already done is fine
		}
	case "pacman":
		if _, err := os.Stat(at("/var/lib/postgres/data/PG_VERSION")); err != nil {
			run(j, "install", "-d", "-o", "postgres", "-g", "postgres", "/var/lib/postgres/data")                                 //nolint:errcheck
			run(j, "runuser", "-u", "postgres", "--", "initdb", "--locale=C.UTF-8", "-E", "UTF8", "-D", "/var/lib/postgres/data") //nolint:errcheck
		}
	}
	if _, err := run(j, "systemctl", "enable", "--now", "postgresql"); err != nil {
		return fmt.Errorf("PostgreSQL didn't start: %w", err)
	}
	if err := waitPG(j, ""); err != nil {
		return err
	}
	if v, _ := psql(j, "", "SHOW server_version_num"); atoi(v) < minPGVersion {
		return fmt.Errorf("this PC has PostgreSQL %d; Karots needs %s or newer — install PostgreSQL %s (postgresql.org/download), then Try again", atoi(v)/10000, pgMajor, pgMajor)
	}
	port := strconv.Itoa(db.Port)
	if cur, _ := psql(j, "", "SHOW port"); cur != port {
		if _, err := psql(j, "", "ALTER SYSTEM SET port = "+port); err != nil {
			return err
		}
		if _, err := run(j, "systemctl", "restart", "postgresql"); err != nil {
			return err
		}
		if err := waitPG(j, port); err != nil {
			return err
		}
	}
	role := "CREATE ROLE"
	if out, _ := psql(j, port, "SELECT 1 FROM pg_roles WHERE rolname = "+lit(db.User)); out == "1" {
		role = "ALTER ROLE"
	}
	if _, err := psql(j, port, role+" "+ident(db.User)+" WITH LOGIN PASSWORD "+lit(db.Password)); err != nil {
		return err
	}
	if out, _ := psql(j, port, "SELECT 1 FROM pg_database WHERE datname = "+lit(db.Name)); out != "1" {
		if _, err := psql(j, port, "CREATE DATABASE "+ident(db.Name)+" OWNER "+ident(db.User)); err != nil {
			return err
		}
	}
	return allowLoopback(j, port)
}

// allowLoopback puts password login for the app first in pg_hba.conf (Fedora /
// Arch default to ident over 127.0.0.1, which refuses the app's password).
func allowLoopback(j *Job, port string) error {
	hba, err := psql(j, port, "SHOW hba_file")
	if err != nil || hba == "" {
		return fmt.Errorf("can't find pg_hba.conf: %v", err)
	}
	cur, err := os.ReadFile(at(hba))
	if err != nil {
		return err
	}
	if strings.Contains(string(cur), hbaMarker) {
		return nil
	}
	db := j.Conf.DB
	rules := fmt.Sprintf("%s: password login for the till over loopback\nhost %s %s 127.0.0.1/32 scram-sha-256\nhost %s %s ::1/128 scram-sha-256\n", hbaMarker, db.Name, db.User, db.Name, db.User)
	if err := os.WriteFile(at(hba), append([]byte(rules), cur...), 0o600); err != nil {
		return err
	}
	_, err = run(j, "systemctl", "reload", "postgresql")
	return err
}

// psql runs one statement as the postgres superuser (peer login), port "" = the default socket.
func psql(j *Job, port, sql string) (string, error) {
	args := []string{"-u", "postgres", "--", "psql"}
	if port != "" {
		args = append(args, "-p", port)
	}
	out, err := run(j, "runuser", append(args, "-tAc", sql)...)
	return strings.TrimSpace(out), err
}

func waitPG(j *Job, port string) error {
	for range 30 {
		if out, _ := psql(j, port, "SELECT 'up'"); out == "up" {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("PostgreSQL doesn't answer — check: systemctl status postgresql")
}

func lit(s string) string   { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func ident(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
