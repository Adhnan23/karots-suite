package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	browser, family := linuxBrowser(j, home)
	if browser == "" {
		return fmt.Errorf("couldn't install Chromium — install a browser (Chromium, Chrome or Firefox), then Try again")
	}
	fmt.Fprintf(j.Log, "kiosk browser: %s\n", browser)
	kiosk := joinOS(j, j.Conf.InstallDir, "kiosk.sh")
	profile := kioskProfile(browser)
	script := fmt.Sprintf(`#!/usr/bin/env bash
# Opens the till full-screen once it answers. Written by karots-setup.
URL=%q
SENTINEL=/tmp/karots-kiosk-exit
mkdir -p "%s"
for _ in $(seq 1 60); do curl -fsS "$URL/health" >/dev/null 2>&1 && break; sleep 2; done
xset s off -dpms s noblank 2>/dev/null || true
# Relaunch loop: Alt+F4 or a crash reopens the till. The only way out is the
# admin / support kiosk-exit hotkey, which writes $SENTINEL and closes the browser.
rm -f "$SENTINEL"
while true; do
  %s %s
  [ -e "$SENTINEL" ] && { rm -f "$SENTINEL"; break; }
  sleep 1
done
`, j.tillURL(), profile, browser, kioskArgs(family, "$URL", profile))
	if err := writeAs(j, kiosk, []byte(script), 0o755); err != nil {
		return err
	}
	// The page can't cancel browser-level F12, so a Chromium kiosk has DevTools
	// off by policy (one of these folders is the one it reads).
	policy := []byte(`{ "DeveloperToolsAvailability": 2 }` + "\n")
	for _, d := range []string{"/etc/chromium/policies/managed", "/etc/chromium-browser/policies/managed", "/var/snap/chromium/common/policies/managed"} {
		if family == famChromium && os.MkdirAll(at(d), 0o755) == nil {
			os.WriteFile(at(filepath.Join(d, "karots-kiosk.json")), policy, 0o644) //nolint:errcheck // best effort per folder
		}
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Karots Suite Kiosk\nComment=Open the till full-screen at sign-in\nExec=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", kiosk)
	return writeAs(j, filepath.Join(home, ".config", "autostart", "karots-kiosk.desktop"), []byte(entry), 0o644)
}

// kioskProfile is the kiosk browser's own profile folder. A snap (Ubuntu's
// Chromium and Firefox) may not touch hidden folders in $HOME — it would fail
// to start forever — so it gets one in the snap's own ~/snap/<name>/common.
func kioskProfile(browser string) string {
	if _, after, ok := strings.Cut(browser, "/snap/bin/"); ok {
		if f := strings.Fields(after); len(f) > 0 {
			if name := strings.Trim(f[0], `"'`); name != "" {
				return "$HOME/snap/" + name + "/common/karots-kiosk"
			}
		}
	}
	return "$HOME/.karots-kiosk"
}

// linuxBrowser is the kiosk browser: the user's default when it's Chrome-style
// or Firefox-style, else one already installed, else Chromium installed now.
// It returns a shell command (quoted path, or the .desktop file's Exec).
func linuxBrowser(j *Job, home string) (cmd, family string) {
	if out, err := j.Run.Run("runuser", "-u", j.User, "--", "xdg-settings", "get", "default-web-browser"); err == nil {
		if name := strings.TrimSpace(out); name != "" {
			if exec := desktopExec(name, home); exec != "" {
				if f := browserFamily(name + " " + exec); f != "" {
					return exec, f
				}
			}
		}
	}
	names := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "brave-browser", "microsoft-edge", "firefox", "librewolf"}
	find := func() (string, string) {
		for _, n := range names {
			if p, err := lookPathFn(n); err == nil {
				return fmt.Sprintf("%q", p), browserFamily(n)
			}
		}
		return "", ""
	}
	if c, f := find(); c != "" {
		return c, f
	}
	if pm, _ := packageManager(); pm == "apt-get" {
		if install(j, "chromium") != nil {
			install(j, "chromium-browser") //nolint:errcheck // tried below
		}
	} else {
		install(j, "chromium") //nolint:errcheck // tried below
	}
	if c, f := find(); c != "" {
		return c, f
	}
	if _, err := lookPathFn("snap"); err == nil {
		if _, err := run(j, "snap", "install", "chromium"); err == nil {
			return `"/snap/bin/chromium"`, famChromium
		}
	}
	return "", ""
}

// desktopExec is the command in a browser's .desktop file, without its %u-style
// field codes ("" when the file isn't found).
func desktopExec(name, home string) string {
	if !strings.HasSuffix(name, ".desktop") || strings.ContainsAny(name, "/\\") {
		return ""
	}
	for _, d := range []string{filepath.Join(home, ".local/share/applications"), "/usr/local/share/applications", "/usr/share/applications",
		"/var/lib/snapd/desktop/applications", "/var/lib/flatpak/exports/share/applications"} {
		raw, err := os.ReadFile(at(filepath.Join(d, name)))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if exec, ok := strings.CutPrefix(strings.TrimSpace(line), "Exec="); ok {
				var keep []string
				for _, f := range strings.Fields(exec) {
					if !strings.HasPrefix(f, "%") {
						keep = append(keep, f)
					}
				}
				return strings.Join(keep, " ")
			}
		}
	}
	return ""
}
