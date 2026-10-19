package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"karots-suite/internal/update"
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
// so the start-up task restarts Karots if it stops. After an update Karots
// exits with update.RestartCode: it is started again at once (the new binary).
func runHidden(args []string) {
	h, err := parseHidden(args)
	if err != nil {
		showError(err.Error())
		os.Exit(2)
	}
	var out *os.File
	if h.log != "" {
		os.MkdirAll(filepath.Dir(h.log), 0o755) //nolint:errcheck // OpenFile says why
		f, err := os.OpenFile(h.log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			showError("can't write " + h.log + ": " + err.Error())
			os.Exit(2)
		}
		defer f.Close()
		out = f
	}
	os.Exit(runChild(func() (int, error) {
		cmd := exec.Command(h.prog, h.args...)
		cmd.Dir = filepath.Dir(h.prog)
		if out != nil {
			cmd.Stdout, cmd.Stderr = out, out
		}
		hideWindow(cmd)
		err := cmd.Run()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return 0, nil
		case errors.As(err, &exit):
			return exit.ExitCode(), nil
		default:
			showError("can't start " + h.prog + ": " + err.Error())
			return 2, err
		}
	}))
}

// crashDelay is the pause before starting Karots again after a crash.
var crashDelay = 5 * time.Second

// runChild keeps the program running: started again at once when it asks
// (update.RestartCode, after an update), after crashDelay when it crashed —
// the scheduled task's restart-on-failure doesn't see an exit code, and
// BootGuard needs the failed starts to roll a bad update back. A clean exit
// (sign-out, shutdown) or a program that can't start at all ends it.
func runChild(start func() (int, error)) int {
	for {
		code, err := start()
		switch {
		case err != nil || code == 0:
			return code
		case code != update.RestartCode:
			time.Sleep(crashDelay)
		}
	}
}
