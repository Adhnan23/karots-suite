//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"
)

func isAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

// elevate starts Setup again through the UAC prompt.
func elevate(exe string, args []string) (wait func() error, err error) {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = windows.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	if err := windows.ShellExecute(0, verb, file, params, nil, windows.SW_HIDE); err != nil {
		return nil, fmt.Errorf("Setup needs admin rights — start it again and answer Yes")
	}
	return func() error { return nil }, nil // the worker runs on its own
}

func openBrowser(url string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() //nolint:errcheck
}

// showError: a windowless Setup (double-clicked) shows errors in a message box.
func showError(msg string) {
	title, _ := windows.UTF16PtrFromString("Karots Setup")
	text, _ := windows.UTF16PtrFromString(msg)
	windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR) //nolint:errcheck
}
