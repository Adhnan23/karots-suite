package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// uninstallSteps removes Karots from this PC: the shop is backed up first (and
// kept, with the automatic backups, in the Karots backups folder), then its
// service, till opening, firewall rule, database and program folder go. Nothing
// is removed when that backup fails. Chrome, curl, CUPS and the like stay.
func uninstallSteps(j *Job) []Step {
	main := func(j *Job) bool { return j.server() }
	all := []Step{
		{ID: "final-backup", Title: "Back up the shop", Applies: func(j *Job) bool { return j.server() && !j.NoBackup }, Do: finalBackup,
			Detail: func(j *Job) string {
				return "Back the shop up to " + j.keepDir() + " (kept, with the automatic backups)"
			}},
		{ID: "remove-startup", Title: "Stop Karots", Applies: main, Do: pick(linuxRemoveStartup, windowsRemoveStartup),
			Detail: func(*Job) string { return "Stop Karots and no longer start it with the PC" }},
		{ID: "remove-open", Title: "Remove the till opening", Applies: always, Do: pick(linuxRemoveOpen, windowsRemoveOpen),
			Detail: func(*Job) string { return "Remove the full-screen till and the Karots Suite shortcuts" }},
		{ID: "remove-network", Title: "Network", Applies: main, Do: pick(linuxRemoveNetwork, windowsRemoveNetwork),
			Detail: func(j *Job) string { return fmt.Sprintf("Close port %d again", j.Conf.Port) }},
		{ID: "remove-database", Title: "Database", Applies: (*Job).localDB, Do: pick(linuxRemoveDatabase, windowsRemoveDatabase),
			Detail: func(j *Job) string {
				if j.RemovePostgres {
					return "Delete the shop's database, then remove PostgreSQL"
				}
				return "Delete the shop's database (PostgreSQL stays)"
			}},
		{ID: "remove-folder", Title: "Remove the program", Applies: main, Do: removeFolder,
			Detail: func(j *Job) string { return "Delete " + j.Conf.InstallDir }},
		{ID: "done", Title: "Done", Applies: always, Do: uninstallDone,
			Detail: func(*Job) string { return "Say where the shop's backup is" }},
	}
	var out []Step
	for _, s := range all {
		if s.Applies(j) {
			out = append(out, s)
		}
	}
	return out
}

// finalBackup stops the till, has the program back the shop up, and copies
// the automatic backups that live inside the program folder.
func finalBackup(j *Job) error {
	if err := safeToRemove(j); err != nil {
		return err
	}
	if err := pick(linuxStop, windowsStop)(j); err != nil && j.windows() {
		return err
	}
	j.note("Backing the shop up to " + j.keepDir())
	path, err := programBackup(j, "final")
	if err != nil {
		return fmt.Errorf("couldn't back the shop up, so nothing was removed: %w", err)
	}
	j.FinalBackup = path
	auto := joinOS(j, j.keepDir(), "automatic")
	for _, dir := range []string{joinOS(j, j.Conf.InstallDir, "data", "backups"), j.Conf.Env["BACKUP_DIR"]} {
		if dir == "" || !inside(j, dir, j.Conf.InstallDir) {
			continue
		}
		entries, _ := os.ReadDir(at(dir))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json.gz") {
				continue
			}
			if err := os.MkdirAll(at(auto), 0o755); err != nil {
				return err
			}
			if err := copyFile(at(joinOS(j, dir, e.Name())), at(joinOS(j, auto, e.Name())), 0o644); err != nil {
				return fmt.Errorf("couldn't keep the automatic backup %s, so nothing was removed: %w", e.Name(), err)
			}
		}
	}
	if !j.windows() {
		run(j, "chown", "-R", j.User+":"+j.User, j.keepDir()) //nolint:errcheck // readable by root either way
	}
	return nil
}

func removeFolder(j *Job) error {
	if err := safeToRemove(j); err != nil {
		return err
	}
	fmt.Fprintf(j.Log, "→ delete %s\n", j.Conf.InstallDir)
	return retryWrite(func() error { return os.RemoveAll(at(j.Conf.InstallDir)) })
}

func uninstallDone(j *Job) error {
	if j.FinalBackup == "" {
		j.note("Karots is removed.")
		return nil
	}
	fmt.Fprintf(j.Log, "backup: %s\n", j.FinalBackup)
	j.note("Karots is removed. The shop's backup is in " + j.FinalBackup + " — keep it: it brings the shop back on any PC.")
	return nil
}

// safeToRemove refuses any folder but the program's own: a full path that
// holds the program, isn't a system or home folder, and doesn't hold the
// Karots backups folder.
func safeToRemove(j *Job) error {
	dir := j.Conf.InstallDir
	clean := strings.TrimRight(strings.ReplaceAll(dir, `\`, "/"), "/")
	low := strings.ToLower(clean)
	home, _ := homeFn(j.User)
	never := []string{"", "/", "/opt", "/usr", "/usr/local", "/home", "/root", "/var", "/etc", "/srv", "/tmp",
		strings.TrimRight(home, "/"), "c:", "c:/program files", "c:/program files (x86)", "c:/users", "c:/windows", "c:/programdata"}
	for _, n := range never {
		if low == strings.ToLower(n) {
			return fmt.Errorf("won't delete %s — it isn't Karots's own folder", dir)
		}
	}
	if !absFor(j.Conf.OS, dir) {
		return fmt.Errorf("won't delete %s — not a full path", dir)
	}
	if _, err := os.Stat(at(joinOS(j, dir, j.Conf.Binary))); err != nil {
		return fmt.Errorf("won't delete %s — Karots (%s) isn't in it", dir, j.Conf.Binary)
	}
	if inside(j, j.keepDir(), dir) {
		return fmt.Errorf("the backups folder %s is inside %s — pick a backups folder outside it", j.keepDir(), dir)
	}
	return nil
}

// inside reports p is dir or under it (case-blind on Windows).
func inside(j *Job, p, dir string) bool {
	norm := func(s string) string {
		s = filepath.ToSlash(strings.ReplaceAll(s, `\`, "/"))
		if j.windows() {
			s = strings.ToLower(s)
		}
		return strings.TrimRight(s, "/")
	}
	p, dir = norm(p), norm(dir)
	return p == dir || strings.HasPrefix(p, dir+"/")
}
