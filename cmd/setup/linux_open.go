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
	// kiosk.sh waits for the till with curl; a fresh Ubuntu has none (without it the
	// wait ran its full two minutes).
	if _, err := lookPathFn("curl"); err != nil {
		install(j, "curl") //nolint:errcheck // kiosk.sh then opens without waiting
	}
	browser, family := linuxBrowser(j, home)
	if browser == "" {
		return fmt.Errorf("couldn't install Chromium — install a browser (Chromium, Chrome or Firefox), then Try again")
	}
	fmt.Fprintf(j.Log, "kiosk browser: %s\n", browser)
	kiosk := joinOS(j, j.Conf.InstallDir, "kiosk.sh")
	profile := "$HOME/.karots-kiosk"
	script := fmt.Sprintf(`#!/usr/bin/env bash
# Opens the till full-screen once it answers. Written by karots-setup.
URL=%q
SENTINEL=/tmp/karots-kiosk-exit
mkdir -p "%s"
command -v curl >/dev/null && for _ in $(seq 1 60); do curl -fsS "$URL/health" >/dev/null 2>&1 && break; sleep 2; done
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
	for _, d := range []string{"/etc/chromium/policies/managed", "/etc/chromium-browser/policies/managed", "/etc/opt/chrome/policies/managed"} {
		if family == famChromium && os.MkdirAll(at(d), 0o755) == nil {
			os.WriteFile(at(filepath.Join(d, "karots-kiosk.json")), policy, 0o644) //nolint:errcheck // best effort per folder
		}
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Karots Suite Kiosk\nComment=Open the till full-screen at sign-in\nExec=%s\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", kiosk)
	return writeAs(j, filepath.Join(home, ".config", "autostart", "karots-kiosk.desktop"), []byte(entry), 0o644)
}

// linuxBrowser is the kiosk browser: the user's default when it's Chrome-style
// or Firefox-style, else one already installed, else one installed now. Only a
// browser installed in the system counts: a snap or flatpak one starts slowly
// in its sandbox and can't use the kiosk's own profile folder (Ubuntu's apt
// "chromium" / "firefox" are snaps). It returns a shell command (quoted path,
// or the .desktop file's Exec).
func linuxBrowser(j *Job, home string) (cmd, family string) {
	if out, err := j.Run.Run("runuser", "-u", j.User, "--", "xdg-settings", "get", "default-web-browser"); err == nil {
		if name := strings.TrimSpace(out); name != "" {
			if exec := desktopExec(name, home); exec != "" && !sandboxed(exec) {
				if f := browserFamily(name + " " + exec); f != "" {
					return exec, f
				}
			}
		}
	}
	names := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "brave-browser", "microsoft-edge", "firefox", "librewolf"}
	find := func() (string, string) {
		for _, n := range names {
			if p, err := lookPathFn(n); err == nil && !sandboxed(p) {
				return fmt.Sprintf("%q", p), browserFamily(n)
			}
		}
		return "", ""
	}
	if c, f := find(); c != "" {
		return c, f
	}
	j.note("Installing a browser for the till — a few minutes")
	if pm, _ := packageManager(); pm != "apt-get" {
		install(j, "chromium") //nolint:errcheck // tried below
	} else if out, _ := run(j, "apt-cache", "show", "--no-all-versions", "chromium"); strings.Contains(out, "Version:") && !strings.Contains(out, "snap") {
		install(j, "chromium") //nolint:errcheck // Debian, Mint: a real package
	} else {
		// Ubuntu: Google Chrome's own .deb (it adds Google's apt source for updates).
		deb := "/tmp/karots-google-chrome.deb"
		if _, err := run(j, "curl", "-fsSL", "-o", deb, "https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb"); err == nil {
			run(j, "apt-get", "install", "-y", "-qq", deb) //nolint:errcheck // tried below
			os.Remove(at(deb))                             //nolint:errcheck
		}
	}
	return find()
}

// sandboxed reports a snap / flatpak browser: its command, where the program
// really lives, or a launcher script (Ubuntu's /usr/bin/firefox) that runs one.
func sandboxed(cmd string) bool {
	if strings.Contains(cmd, "/snap/") || strings.Contains(cmd, "flatpak") {
		return true
	}
	for _, f := range strings.Fields(cmd) {
		p := strings.Trim(f, `"'`)
		if p == "env" || strings.Contains(p, "=") || strings.HasPrefix(p, "-") {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			var err error
			if p, err = lookPathFn(p); err != nil {
				return false
			}
		}
		if real, err := filepath.EvalSymlinks(at(p)); err == nil && strings.Contains(real, "/snap/") {
			return true
		}
		head := make([]byte, 4096)
		fh, err := os.Open(at(p))
		if err != nil {
			return false
		}
		n, _ := fh.Read(head)
		fh.Close()
		s := string(head[:n])
		return strings.HasPrefix(s, "#!") && (strings.Contains(s, "/snap/") || strings.Contains(s, "snap run") || strings.Contains(s, "flatpak"))
	}
	return false
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
