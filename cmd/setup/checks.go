package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Swapped in tests.
var (
	isAdminFn    = isAdmin
	hasSystemdFn = func() bool { _, err := os.Stat("/run/systemd/system"); return err == nil }
)

// checks makes sure this PC can take the install before anything changes.
func checks(j *Job) error {
	if !isAdminFn() {
		return fmt.Errorf("Setup needs admin rights — start it again and answer Yes")
	}
	if !j.windows() && j.User == "" {
		return fmt.Errorf("start Setup from your normal user (sudo ./karots-setup), not as root — Karots runs as that user")
	}
	if j.Role == RoleCashier {
		return nil
	}
	if !j.windows() && !hasSystemdFn() {
		return fmt.Errorf("this Linux has no systemd, so Karots can't start with the PC")
	}
	if _, err := os.Stat(filepath.Join(j.Dir, j.Conf.Binary)); err != nil {
		return fmt.Errorf("%s isn't next to karots-setup — copy all three files from the Shop Builder", j.Conf.Binary)
	}
	for _, old := range []string{"karots-pos", "karots-pos.exe"} {
		if _, err := os.Stat(filepath.Join(j.Conf.InstallDir, old)); err == nil {
			return fmt.Errorf("%s holds karots-pos — pick another install folder in the Shop Builder", j.Conf.InstallDir)
		}
	}
	// Our own install holds the port during an Update; the program step stops it.
	if !j.Updating {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(j.Conf.Port))
		if err != nil {
			return fmt.Errorf("port %d is used by another program (karots-pos?) — stop it, or change the shop's till port in the Shop Builder", j.Conf.Port)
		}
		ln.Close()
	}
	return nil
}

// health waits for the till to answer /health — up to 90 s: a one-PC shop's
// first start creates its database (slow on an old PC or a VM).
func health(j *Job) error {
	url := j.tillURL() + "/health"
	c := http.Client{Timeout: 2 * time.Second}
	for range 90 {
		if res, err := c.Get(url); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				fmt.Fprintln(j.Log, "the till answers at", j.tillURL())
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("the till doesn't answer at %s yet — restart this PC, then run Setup again; if it still fails, Copy log for support", j.tillURL())
}
