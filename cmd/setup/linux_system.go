package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// Swapped in tests: root prefixes every path Setup writes; lookPathFn and
// homeFn stand in for the PC's programs and the shop user's home.
var (
	root       = ""
	lookPathFn = exec.LookPath
	homeFn     = func(name string) (string, error) {
		u, err := user.Lookup(name)
		if err != nil {
			return "", err
		}
		return u.HomeDir, nil
	}
)

const linuxService = "karots-suite"

// at is p under root (the real filesystem outside tests).
func at(p string) string { return filepath.Join(root, p) }

// run runs a tool, logs it, and wraps a failure with its output.
func run(j *Job, name string, args ...string) (string, error) {
	fmt.Fprintf(j.Log, "→ %s %s\n", name, j.redact(strings.Join(args, " ")))
	out, err := j.Run.Run(name, args...)
	if strings.TrimSpace(out) != "" {
		fmt.Fprintln(j.Log, strings.TrimSpace(out))
	}
	if err != nil {
		return out, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

// redact keeps the job's passwords out of the log (support gets a copy of it).
func (j *Job) redact(s string) string {
	var secrets []string
	if j.Conf.DB != nil {
		secrets = append(secrets, j.Conf.DB.Password, j.Conf.DB.SuperPassword)
	}
	for _, v := range append(secrets, j.SuperPassword) {
		if len(v) >= 3 {
			s = strings.ReplaceAll(s, v, "••••")
		}
	}
	return s
}

// writeAs writes a file and hands it to the shop user.
func writeAs(j *Job, path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(at(path)), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(at(path), data, mode); err != nil {
		return err
	}
	if err := os.Chmod(at(path), mode); err != nil {
		return err
	}
	_, err := run(j, "chown", j.User+":"+j.User, path)
	return err
}

// packageManager is the distro's installer: apt, dnf, pacman or zypper.
func packageManager() (string, error) {
	for _, pm := range []string{"apt-get", "dnf", "pacman", "zypper"} {
		if _, err := lookPathFn(pm); err == nil {
			return pm, nil
		}
	}
	return "", fmt.Errorf("this Linux has none of apt, dnf, pacman or zypper — install the programs by hand, then run Setup again")
}

func install(j *Job, pkgs ...string) error {
	pm, err := packageManager()
	if err != nil {
		return err
	}
	switch pm {
	case "apt-get":
		if _, err := run(j, "apt-get", "update", "-qq"); err != nil {
			return err
		}
		_, err = run(j, "apt-get", append([]string{"install", "-y", "-qq"}, pkgs...)...)
	case "dnf":
		_, err = run(j, "dnf", append([]string{"install", "-y", "-q"}, pkgs...)...)
	case "pacman":
		_, err = run(j, "pacman", append([]string{"-S", "--noconfirm", "--needed"}, pkgs...)...)
	case "zypper":
		_, err = run(j, "zypper", append([]string{"--non-interactive", "install", "-y"}, pkgs...)...)
	}
	return err
}

func linuxProgram(j *Job) error {
	if j.Updating {
		if _, err := run(j, "systemctl", "stop", linuxService); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(at(j.Conf.InstallDir), 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(j.Dir, j.Conf.Binary), at(j.binPath()), 0o755); err != nil {
		return fmt.Errorf("copy %s: %w", j.Conf.Binary, err)
	}
	_, err := run(j, "chown", "-R", j.User+":"+j.User, j.Conf.InstallDir)
	return err
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst) // replaces a stopped binary in one step
}

func linuxSettings(j *Job) error {
	if err := writeAs(j, joinOS(j, j.Conf.InstallDir, ".env"), j.Conf.EnvFile(), 0o600); err != nil {
		return err
	}
	return saveChoices(j)
}

func linuxBackups(j *Job) error {
	dir := j.backupDir()
	if err := os.MkdirAll(at(dir), 0o755); err != nil {
		return err
	}
	_, err := run(j, "chown", j.User+":"+j.User, dir)
	return err
}

func linuxStartup(j *Job) error {
	after := "network-online.target"
	if j.localDB() {
		after += " postgresql.service"
	}
	unit := fmt.Sprintf(`[Unit]
Description=Karots Suite — %s
After=%s
Wants=%s

[Service]
Type=simple
User=%s
WorkingDirectory=%s
EnvironmentFile=%s
ExecStart=%s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, j.Conf.Shop, after, after, j.User, j.Conf.InstallDir, joinOS(j, j.Conf.InstallDir, ".env"), j.binPath())
	path := at("/etc/systemd/system/" + linuxService + ".service")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", linuxService}, {"restart", linuxService}} {
		if _, err := run(j, "systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

// linuxNetwork opens the till port when ufw or firewalld is on; neither on = nothing to open.
func linuxNetwork(j *Job) error {
	port := fmt.Sprintf("%d/tcp", j.Conf.Port)
	if _, err := lookPathFn("ufw"); err == nil {
		if out, _ := j.Run.Run("ufw", "status"); strings.Contains(out, "Status: active") {
			_, err := run(j, "ufw", "allow", port)
			return err
		}
	}
	if _, err := lookPathFn("firewall-cmd"); err == nil {
		if _, err := j.Run.Run("systemctl", "is-active", "--quiet", "firewalld"); err == nil {
			if _, err := run(j, "firewall-cmd", "--add-port="+port, "--permanent"); err != nil {
				return err
			}
			_, err := run(j, "firewall-cmd", "--reload")
			return err
		}
	}
	fmt.Fprintln(j.Log, "no firewall is on — nothing to open")
	return nil
}

func linuxPrinting(j *Job) error {
	if _, err := lookPathFn("cupsd"); err != nil {
		if err := install(j, "cups"); err != nil {
			return err
		}
	}
	_, err := run(j, "systemctl", "enable", "--now", "cups")
	return err
}
