package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// kioskSentinel returns the file the kiosk relaunch loop watches: when it exists
// after the browser quits, the loop breaks and stays on the desktop instead of
// reopening the till. It MUST match the path baked into the launcher for this OS
// — kiosk.sh (Linux) uses /tmp, kiosk.cmd (Windows) uses %TEMP%. The server and
// the launcher run as the same desktop user, so both resolve to the same file.
func kioskSentinel() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "karots-kiosk-exit")
	}
	return "/tmp/karots-kiosk-exit"
}

// killKiosk signals the kiosk browser to quit — precisely, matching the --kiosk
// flag so a support user's own browser windows are left alone. Best-effort: no
// kiosk running (dev, or already closed) is not an error.
//
// ponytail: matches on the "--kiosk" flag, so it's fine for one kiosk per box;
// revisit only if a machine ever runs two.
func killKiosk() {
	if runtime.GOOS == "windows" {
		// taskkill can't filter on the command line, so find the kiosk browser by
		// its --kiosk arg via CIM and stop just those PIDs.
		const ps = `Get-CimInstance Win32_Process -Filter "Name='chrome.exe' OR Name='chromium.exe' OR Name='msedge.exe'" | ` +
			`Where-Object { $_.CommandLine -like '*--kiosk*' } | ` +
			`ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`
		_ = exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
		return
	}
	// The "[-]-kiosk" regex matches the "--kiosk" flag but not pkill's own cmdline.
	_ = exec.Command("pkill", "-f", "[-]-kiosk").Run()
}

func (h *handlers) registerKiosk(v1 *echo.Group, jwt echo.MiddlewareFunc) {
	v1.POST("/kiosk/exit", h.KioskExit, jwt, middleware.RequireKioskExit())
}

// KioskExit: write the relaunch loop's exit sentinel, then kill the --kiosk
// browser. Admin/support only (RequireKioskExit 404s everyone else).
func (h *handlers) KioskExit(c echo.Context) error {
	if err := os.WriteFile(kioskSentinel(), []byte("1"), 0o644); err != nil {
		return err
	}
	killKiosk()
	return c.NoContent(http.StatusNoContent)
}
