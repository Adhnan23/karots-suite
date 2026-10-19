//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func isAdmin() bool { return os.Geteuid() == 0 }

// elevate starts Setup again through pkexec (the desktop's password box).
func elevate(exe string, args []string) (wait func() error, err error) {
	if _, err := exec.LookPath("pkexec"); err != nil {
		return nil, fmt.Errorf("Setup needs admin rights: open a terminal in this folder and run  sudo ./karots-setup")
	}
	cmd := exec.Command("pkexec", append([]string{exe}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Wait, nil
}

// openBrowser opens the page as the person at the desktop — a browser won't
// run as root, so under sudo it goes through the user who ran sudo.
func openBrowser(url string) {
	if os.Geteuid() == 0 {
		if u := shopUser(); u != "" {
			exec.Command("runuser", "-u", u, "--", "xdg-open", url).Start() //nolint:errcheck
			return
		}
	}
	exec.Command("xdg-open", url).Start() //nolint:errcheck
}

// showError tells someone who double-clicked (no terminal) what went wrong.
func showError(msg string) {
	fmt.Fprintln(os.Stderr, "karots-setup: "+msg)
	for _, c := range [][]string{{"zenity", "--error", "--title=Karots Setup", "--text=" + msg}, {"kdialog", "--title", "Karots Setup", "--error", msg}} {
		if _, err := exec.LookPath(c[0]); err == nil {
			exec.Command(c[0], c[1:]...).Run() //nolint:errcheck
			return
		}
	}
}
