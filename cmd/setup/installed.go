package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"karots-suite/internal/shopconf"
)

// What an Update keeps from this PC. The Builder owns the database, secret,
// ports and its More settings (install.conf has them — a Rotate must reach the
// shop); the PC owns its backup choices, kiosk / background and any line added
// to .env by hand.
var pcOwned = []string{"BACKUP_DIR", "BACKUP_INTERVAL", "BACKUP_KEEP"}

// builderOwned is never kept from this PC, even when install.conf leaves it
// out: a server → file Update that kept DATABASE_URL stayed on the server.
var builderOwned = []string{"DATABASE_URL"}

// choicesFile remembers what was picked on this PC (kiosk / background).
const choicesFile = "setup.json"

// readEnvFile reads KEY=value lines (nil when there is no file).
func readEnvFile(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			continue
		}
		env[k] = strings.TrimSpace(v)
	}
	return env
}

// installedOpen is how this PC opens the till now: what Setup recorded, else
// guessed from a kiosk script being there ("" = unknown → the Builder's).
func installedOpen(installDir string) string {
	var ch struct{ Open string }
	if raw, err := os.ReadFile(filepath.Join(installDir, choicesFile)); err == nil && json.Unmarshal(raw, &ch) == nil && ch.Open != "" {
		return ch.Open
	}
	for _, k := range []string{"kiosk.cmd", "kiosk.sh"} {
		if _, err := os.Stat(filepath.Join(installDir, k)); err == nil {
			return "kiosk"
		}
	}
	return ""
}

// applyInstalled lays this PC's current choices over install.conf for an Update
// and says what it kept (the page shows "Kept from this PC").
func applyInstalled(c shopconf.Conf, installed map[string]string, open string) (shopconf.Conf, []string) {
	env := make(map[string]string, len(c.Env)+len(installed))
	for k, v := range c.Env {
		env[k] = v
	}
	var kept []string
	for k, v := range installed {
		if slices.Contains(builderOwned, k) {
			continue
		}
		_, builder := c.Env[k]
		pc := false
		for _, o := range pcOwned {
			pc = pc || o == k
		}
		if (pc || !builder) && v != "" && env[k] != v {
			env[k] = v
			kept = append(kept, k)
		}
	}
	sort.Strings(kept)
	c.Env = env
	if open == "kiosk" || open == "background" {
		if open != c.Open {
			kept = append(kept, "start: "+open)
		}
		c.Open = open
	}
	return c, kept
}

// saveChoices records this PC's kiosk / background pick for the next Update.
func saveChoices(j *Job) error {
	raw, _ := json.Marshal(map[string]string{"open": j.Conf.Open})
	return os.WriteFile(at(joinOS(j, j.Conf.InstallDir, choicesFile)), raw, 0o644)
}

// installedMode is how the installed shop is kept: "server" (its .env has a
// DATABASE_URL), "file" (no DATABASE_URL, a data folder beside the program) or
// "" (not installed / can't tell).
func installedMode(installDir string) string {
	env := readEnvFile(filepath.Join(installDir, ".env"))
	if env["DATABASE_URL"] != "" {
		return "server"
	}
	if st, err := os.Stat(filepath.Join(installDir, "data")); env != nil && err == nil && st.IsDir() {
		return "file"
	}
	return ""
}
