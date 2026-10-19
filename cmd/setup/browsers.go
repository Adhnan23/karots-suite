package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Kiosk browsers come in three families, each with its own full-screen form.
// Anything else (an unknown default browser) is passed over.
const (
	famChromium = "chromium" // Chrome, Chromium, Brave, Vivaldi, Helium, Opera…
	famEdge     = "edge"
	famFirefox  = "firefox" // Firefox, LibreWolf, Waterfox, Floorp, Zen…
)

// browserFamily names a browser's family from its program or .desktop name.
func browserFamily(name string) string {
	n := strings.ToLower(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	switch {
	case strings.Contains(n, "msedge") || strings.Contains(n, "microsoft-edge"):
		return famEdge
	case containsAny(n, "firefox", "librewolf", "waterfox", "floorp", "zen-browser", "zen.", "mullvad"):
		return famFirefox
	case containsAny(n, "chrom", "brave", "vivaldi", "helium", "opera", "thorium", "ungoogled"):
		return famChromium
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// pickBrowser is the default browser when its family is known, else the first
// known one found on the PC.
func pickBrowser(def string, found []string) (path, family string) {
	for _, b := range append([]string{def}, found...) {
		if b == "" {
			continue
		}
		if f := browserFamily(b); f != "" {
			return b, f
		}
	}
	return "", ""
}

// kioskArgs is the browser's own full-screen, can't-leave form for the till.
// profile is the kiosk's own browser profile (so a browser the owner already
// has open can't swallow the kiosk window).
func kioskArgs(family, url, profile string) string {
	switch family {
	case famFirefox:
		return fmt.Sprintf(`--kiosk -no-remote -profile "%s" %s`, profile, url)
	case famEdge:
		return fmt.Sprintf(`--kiosk %s --edge-kiosk-type=fullscreen --inprivate --user-data-dir="%s" --no-first-run --noerrdialogs`, url, profile)
	}
	return fmt.Sprintf(`--kiosk --app=%s --incognito --user-data-dir="%s" --no-first-run --noerrdialogs --disable-infobars `+
		`--disable-session-crashed-bubble --disable-features=TranslateUI --check-for-update-interval=31536000 `+
		`--overscroll-history-navigation=0 --password-store=basic`, url, profile)
}
