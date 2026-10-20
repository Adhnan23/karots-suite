package main

import (
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
