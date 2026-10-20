package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// keepDir is the Karots backups folder: the uninstall's final backup, a mode
// change's before-move backup, and where a first install looks for one to bring
// back. Somewhere obvious, outside the program folder (which an uninstall removes).
func (j *Job) keepDir() string {
	if j.KeepDir != "" {
		return j.KeepDir
	}
	if j.windows() {
		return `C:\Karots backups`
	}
	home, err := homeFn(j.User)
	if err != nil || home == "" {
		home = "/root"
	}
	return filepath.Join(home, "Karots backups")
}

// programBackup has the installed program write one full backup (tag "final",
// "before-move") into keepDir and returns its path. The till must be stopped.
func programBackup(j *Job, tag string) (string, error) {
	dir := j.keepDir()
	if err := os.MkdirAll(at(dir), 0o755); err != nil {
		return "", err
	}
	if !j.windows() {
		if _, err := run(j, "chown", j.User+":"+j.User, dir); err != nil {
			return "", err
		}
	}
	return programLine(j, "backup: ", "-backup", dir, "-tag", tag)
}

// programRestore has the installed program migrate and bring the backup back.
func programRestore(j *Job, file string) (string, error) {
	line, err := programLine(j, "restored: ", "-restore", file)
	if err != nil {
		return "", err
	}
	return "restored: " + line, nil
}

// programLine runs the installed program (as the shop user on Linux: file mode
// refuses root) and returns what follows prefix on its answer line.
func programLine(j *Job, prefix string, args ...string) (string, error) {
	name, cmd := j.binPath(), args
	if !j.windows() {
		name, cmd = "runuser", append([]string{"-u", j.User, "--", j.binPath()}, args...)
	}
	out, err := run(j, name, cmd...)
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok && err == nil {
			return v, nil
		}
	}
	if err == nil {
		err = fmt.Errorf("Karots didn't say it was done")
	}
	return "", fmt.Errorf("%w: %s", err, oneLine(lastLines(out)))
}

// lastLines is the end of a program's output (its error is last).
func lastLines(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

type foundBackup struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Modified    time.Time `json:"modified"`
	BesideSetup bool      `json:"besideSetup"` // put next to the three files: "bring this one back"
}

// findBackups lists the backups in keepDir and beside Setup, newest first.
func findBackups(j *Job) []foundBackup {
	var out []foundBackup
	add := func(dir, real string, beside bool) {
		entries, _ := os.ReadDir(real)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json.gz") {
				continue
			}
			if info, err := e.Info(); err == nil {
				out = append(out, foundBackup{joinOS(j, dir, e.Name()), e.Name(), info.ModTime(), beside})
			}
		}
	}
	add(j.keepDir(), at(j.keepDir()), false)
	add(j.Dir, j.Dir, true)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Modified.After(out[b].Modified) })
	return out
}

// backupName is a backup's file name, from a Linux or Windows path.
func backupName(p string) string { return filepath.Base(strings.ReplaceAll(p, `\`, "/")) }

// safetyBackup is a mode change's first step: stop the till and have the old
// program back the shop up, before anything changes. The restore step brings
// that backup back with the new program.
func safetyBackup(j *Job) error {
	if err := pick(linuxStop, windowsStop)(j); err != nil {
		return err
	}
	j.note("Backing the shop up to " + j.keepDir())
	path, err := programBackup(j, "before-move")
	if err != nil {
		return fmt.Errorf("couldn't back the shop up, so nothing was changed: %w", err)
	}
	fmt.Fprintf(j.Log, "backup: %s\n", path)
	j.Restore = path
	return savePending(j)
}

// restoreStep brings the shop back into the new install (before it starts with
// the PC). After a file → server change the old data folder is set aside, not
// deleted — the backup and that folder can go once the shop looks right.
func restoreStep(j *Job) error {
	if j.Restore == "" {
		return fmt.Errorf("no backup to bring back")
	}
	if err := savePending(j); err != nil {
		return err
	}
	j.note("Bringing the shop back from " + backupName(j.Restore))
	summary, err := programRestore(j, j.Restore)
	if err != nil {
		return fmt.Errorf("couldn't bring the shop back from %s: %w", j.Restore, err)
	}
	j.note(summary)
	os.Remove(at(joinOS(j, j.Conf.InstallDir, pendingFile)))
	if j.FromMode != "file" {
		return nil
	}
	old := joinOS(j, j.Conf.InstallDir, "data")
	if _, err := os.Stat(at(old)); err != nil {
		return nil
	}
	aside := joinOS(j, j.Conf.InstallDir, "data-before-server-"+time.Now().Format("2006-01-02"))
	for n := 2; ; n++ {
		if _, err := os.Stat(at(aside)); err != nil {
			break
		}
		aside = joinOS(j, j.Conf.InstallDir, fmt.Sprintf("data-before-server-%s-%d", time.Now().Format("2006-01-02"), n))
	}
	fmt.Fprintf(j.Log, "old data kept in %s\n", aside)
	return os.Rename(at(old), at(aside))
}

// pendingFile marks a restore not done yet. Once the new settings are written
// a retry can't see the mode change any more, so without it "Try again"
// started the till on an empty database.
const pendingFile = "restore-pending.json"

type pendingRestore struct {
	Restore  string `json:"restore"`
	FromMode string `json:"fromMode"`
}

func savePending(j *Job) error {
	raw, _ := json.Marshal(pendingRestore{j.Restore, j.FromMode})
	if err := os.MkdirAll(at(j.Conf.InstallDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(at(joinOS(j, j.Conf.InstallDir, pendingFile)), raw, 0o644)
}

// applyPending picks up a restore an earlier run didn't finish: it runs again
// (no second safety backup — that one would hold the new, empty database).
func applyPending(j *Job) {
	raw, err := os.ReadFile(at(joinOS(j, j.Conf.InstallDir, pendingFile)))
	var p pendingRestore
	if err != nil || json.Unmarshal(raw, &p) != nil || p.Restore == "" {
		return
	}
	j.Restore, j.pendingRestore, j.ChangeMode = p.Restore, true, true
	if j.FromMode == "" {
		j.FromMode = p.FromMode
	}
}
