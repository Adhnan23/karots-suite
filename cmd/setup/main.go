// Command karots-setup installs a shop from the three files the Shop Builder
// makes: karots-setup itself, install.conf and the shop's binary. Double-click
// it: a page opens, press Install. It only applies what the Builder decided —
// it never makes a secret or changes a setting.
//
//	karots-setup                     the page (asks for admin rights first)
//	sudo ./karots-setup -yes         headless: install as the shop's main PC, in the terminal
//	sudo ./karots-setup -yes -role cashier -address 192.168.1.10:3000
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "karots-setup: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		yes     = flag.Bool("yes", false, "install without the page (a server with no screen)")
		role    = flag.String("role", "server", "with -yes: server (the shop's main PC) | cashier")
		address = flag.String("address", "", "with -role cashier: the main PC's host[:port]")
	)
	flag.Parse()
	j, err := newJob(Role(*role), *address)
	if err != nil {
		return err
	}
	if *yes {
		return headless(j)
	}
	return fmt.Errorf("the page arrives in a later step — use -yes for now")
}

// newJob reads install.conf from beside the program and fills in this PC.
func newJob(role Role, address string) (*Job, error) {
	dir, err := exeDir()
	if err != nil {
		return nil, err
	}
	c, err := loadConf(dir)
	if err != nil {
		return nil, err
	}
	if role != RoleServer && role != RoleCashier {
		return nil, fmt.Errorf("role %q (server | cashier)", role)
	}
	j := &Job{Conf: c, Role: role, Dir: dir, Run: osRunner{}, Log: os.Stdout, User: shopUser()}
	if role == RoleCashier {
		if j.Address = withPort(address, c.Port); j.Address == "" {
			if j.Address = withPort(c.LanAddress, c.Port); j.Address == "" {
				return nil, fmt.Errorf("a cashier PC needs the main PC's address (-address 192.168.1.10)")
			}
		}
	}
	_, err = os.Stat(filepath.Join(c.InstallDir, c.Binary))
	j.Updating = err == nil
	return j, nil
}

// withPort adds the till port when the address has none.
func withPort(addr string, port int) string {
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	addr = strings.TrimRight(addr, "/")
	if addr == "" || strings.Contains(addr, ":") {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func headless(j *Job) error {
	fmt.Printf("Karots Suite — installing %s (%s)\n", j.Conf.Shop, j.Role)
	err := RunAll(j, func(id string, err error) {
		if err != nil {
			fmt.Printf("✗ %s: %v\n", id, err)
		} else {
			fmt.Printf("✓ %s\n", id)
		}
	})
	if err == nil {
		fmt.Printf("\nDone. The till: %s   (install id %s — support reads the PIN from it)\n", j.tillURL(), j.Conf.InstallID)
	}
	return err
}

func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Dir(exe), nil
}

// shopUser is who the service, kiosk and shortcuts belong to: the person who
// started Setup, not root / the elevated admin.
func shopUser() string {
	for _, k := range []string{"KAROTS_SETUP_USER", "SUDO_USER"} {
		if u := os.Getenv(k); u != "" && u != "root" {
			return u
		}
	}
	if os.Getenv("USERDOMAIN") != "" {
		return os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	}
	if u := os.Getenv("USER"); u != "root" {
		return u
	}
	return ""
}

// osRunner runs the real tools, logging each command and its output.
type osRunner struct{}

func (osRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func (r osRunner) Script(ps string) (string, error) {
	return r.Run("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
}
