package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// linuxOpen makes the till open at sign-in: full-screen kiosk (karots-pos's
// relaunch loop + DevTools policy), or — on an owner's own PC — nothing locked:
// a Karots Suite shortcut on the desktop and in the app menu.
func linuxOpen(j *Job) error {
	home, err := homeFn(j.User)
	if err != nil {
		return fmt.Errorf("can't find %s's home folder: %w", j.User, err)
	}
	if j.Role == RoleServer && j.Conf.Open == "background" {
		entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Karots Suite\nComment=Open the till\nExec=xdg-open %s\nIcon=applications-office\nTerminal=false\n", j.tillURL())
		for _, p := range []string{filepath.Join(home, "Desktop", "karots-suite.desktop"), filepath.Join(home, ".local", "share", "applications", "karots-suite.desktop")} {
			if err := writeAs(j, p, []byte(entry), 0o755); err != nil {
				return err
			}
		}
		return nil
	}
	browser := ""
	for _, b := range []string{"chromium", "chromium-browser"} {
		if p, err := lookPathFn(b); err == nil {
			browser = p
			break
		}
	}
	if browser == "" {
		if pm, _ := packageManager(); pm == "apt-get" {
			if install(j, "chromium") != nil {
				install(j, "chromium-browser") //nolint:errcheck // tried below
			}
		} else {
			install(j, "chromium") //nolint:errcheck // tried below
		}
		for _, b := range []string{"chromium", "chromium-browser"} {
			if p, err := lookPathFn(b); err == nil {
				browser = p
				break
			}
		}
		if browser == "" {
			if _, err := lookPathFn("snap"); err == nil {
				if _, err := run(j, "snap", "install", "chromium"); err == nil {
					browser = "/snap/bin/chromium"
				}
			}
		}
	}
	if browser == "" {
		return fmt.Errorf("couldn't install Chromium — install it from the app store, then run Setup again")
	}
	kiosk := joinOS(j, j.Conf.InstallDir, "kiosk.sh")
	script := fmt.Sprintf(`#!/usr/bin/env bash
# Opens the till full-screen once it answers. Written by karots-setup.
URL=%q
SENTINEL=/tmp/karots-kiosk-exit
for _ in $(seq 1 60); do curl -fsS "$URL/health" >/dev/null 2>&1 && break; sleep 2; done
xset s off -dpms s noblank 2>/dev/null || true
# Relaunch loop: Alt+F4 or a crash reopens the till. The only way out is the
# admin / support kiosk-exit hotkey, which writes $SENTINEL and closes the browser.
rm -f "$SENTINEL"
while true; do
  %q --kiosk --app="$URL" --incognito \
    --noerrdialogs --disable-infobars --disable-session-crashed-bubble \
    --disable-features=TranslateUI --check-for-update-interval=31536000 \
    --overscroll-history-navigation=0 --password-store=basic
  [ -e "$SENTINEL" ] && { rm -f "$SENTINEL"; break; }
  sleep 1
done
`, j.tillURL(), browser)
	if err := writeAs(j, kiosk, []byte(script), 0o755); err != nil {
		return err
	}
	// The page can't cancel browser-level F12, so DevTools are off by policy
	// (one of these folders is the one this Chromium reads).
	policy := []byte(`{ "DeveloperToolsAvailability": 2 }` + "\n")
	for _, d := range []string{"/etc/chromium/policies/managed", "/etc/chromium-browser/policies/managed", "/var/snap/chromium/common/policies/managed"} {
		if os.MkdirAll(at(d), 0o755) == nil {
			os.WriteFile(at(filepath.Join(d, "karots-kiosk.json")), policy, 0o644) //nolint:errcheck // best effort per folder
		}
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Karots Suite Kiosk\nComment=Open the till full-screen at sign-in\nExec=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", kiosk)
	return writeAs(j, filepath.Join(home, ".config", "autostart", "karots-kiosk.desktop"), []byte(entry), 0o644)
}
