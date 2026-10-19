package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

// -hidden: karots-start.exe (Setup's own copy in the install folder; built
// without a console) starts Karots or the kiosk loop with no window, so
// nothing pops up at sign-in:
//
//	karots-start.exe -hidden -log C:\karots-suite\logs\karots.log C:\karots-suite\karots-suite.exe
//	karots-start.exe -hidden cmd.exe /c C:\karots-suite\kiosk.cmd
type hidden struct {
	log, prog string
	args      []string
}

func parseHidden(args []string) (hidden, error) {
	var h hidden
	if len(args) >= 2 && args[0] == "-log" {
		h.log, args = args[1], args[2:]
	}
	if len(args) == 0 {
		return h, errors.New("-hidden: nothing to run")
	}
	h.prog, h.args = args[0], args[1:]
	return h, nil
}

// runHidden runs the program (no window — hideWindow) and exits with its code,
// so the start-up task restarts Karots if it stops.
func runHidden(args []string) {
	h, err := parseHidden(args)
	if err != nil {
		showError(err.Error())
		os.Exit(2)
	}
	cmd := exec.Command(h.prog, h.args...)
	cmd.Dir = filepath.Dir(h.prog)
	if h.log != "" {
		os.MkdirAll(filepath.Dir(h.log), 0o755) //nolint:errcheck // OpenFile says why
		f, err := os.OpenFile(h.log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			showError("can't write " + h.log + ": " + err.Error())
			os.Exit(2)
		}
		defer f.Close()
		cmd.Stdout, cmd.Stderr = f, f
	}
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		showError("can't start " + h.prog + ": " + err.Error())
		os.Exit(2)
	}
}
