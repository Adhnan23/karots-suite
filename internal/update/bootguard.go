package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Swap puts newPath in place of exe, keeping the old binary as exe.old. It
// works while exe is running. On Linux the old one is kept by a second name and
// the new one renamed over it in one step, so a power cut at any instant leaves
// a program in place (UPD-4). Windows can't replace a running .exe, only rename
// it: the old one is moved aside first, and Recover (run by the launcher before
// each start) puts a program back if the power went between the two.
func Swap(exe, newPath string) error {
	old := exe + ".old"
	os.Remove(old)
	if runtime.GOOS != "windows" {
		if err := keepAs(exe, old); err != nil {
			return fmt.Errorf("can't keep the current program: %w", err)
		}
		if err := os.Rename(newPath, exe); err != nil {
			return fmt.Errorf("can't put the new program in place: %w", err)
		}
		return nil
	}
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("can't move the current program aside: %w", err)
	}
	if err := os.Rename(newPath, exe); err != nil {
		os.Rename(old, exe)
		return fmt.Errorf("can't put the new program in place: %w", err)
	}
	return nil
}

// keepAs gives exe a second name (a hard link; a copy where links don't work).
func keepAs(exe, name string) error {
	if os.Link(exe, name) == nil {
		return nil
	}
	in, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(name+".tmp", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(name+".tmp", name)
}

// Recover puts a program back when a power cut left none in place (Windows:
// between a swap's or a rollback's two renames): the previous one, else the
// new one, else the one that failed. Nothing happens while exe is there.
func Recover(exe string) {
	if _, err := os.Stat(exe); !errors.Is(err, os.ErrNotExist) {
		return
	}
	for _, suf := range []string{".old", ".new", ".failed"} {
		if os.Rename(exe+suf, exe) == nil {
			os.Remove(markerPath(exe)) // whatever was pending didn't land
			return
		}
	}
}

func markerPath(exe string) string { return filepath.Join(filepath.Dir(exe), "update-pending") }

// markPending starts counting the new binary's boots: "<version>\n<boots>".
func markPending(exe, version string) error {
	return os.WriteFile(markerPath(exe), []byte(version+"\n0"), 0o644)
}

// BootGuard runs first thing at startup. After an update, every boot of the
// new binary is counted; if it has failed to stay up twice (crash, failed
// migration, …) the previous binary is put back, the version that failed is
// written to <exe>.rolledback, and BootGuard reports true — the caller then
// exits with RestartCode so the supervisor starts the old binary. A marker for
// another version than current means that update never landed (the power went
// before the swap): it is dropped, and this program's boots aren't counted.
func BootGuard(exe, current string) (rolledBack bool, err error) {
	marker := markerPath(exe)
	b, err := os.ReadFile(marker)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	version, count, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if version != current {
		os.Remove(marker)
		return false, nil
	}
	boots, _ := strconv.Atoi(strings.TrimSpace(count))
	if boots < 2 {
		return false, os.WriteFile(marker, []byte(version+"\n"+strconv.Itoa(boots+1)), 0o644)
	}
	old := exe + ".old"
	if _, err := os.Stat(old); err != nil {
		os.Remove(marker) // nothing to go back to
		return false, nil
	}
	os.Remove(exe + ".failed")
	if runtime.GOOS != "windows" { // one step, as Swap (UPD-4)
		if err := keepAs(exe, exe+".failed"); err != nil {
			return false, err
		}
		if err := os.Rename(old, exe); err != nil {
			return false, err
		}
	} else {
		if err := os.Rename(exe, exe+".failed"); err != nil {
			return false, err
		}
		if err := os.Rename(old, exe); err != nil {
			os.Rename(exe+".failed", exe)
			return false, err
		}
	}
	os.Remove(marker)
	os.WriteFile(exe+".rolledback", []byte(version), 0o644) //nolint:errcheck // only the Data page's notice
	return true, nil
}

// MarkHealthy clears the boot counter once the binary running as version has
// stayed up. Only its own marker: the old binary's timer may fire after it has
// just installed an update, whose boots must still be counted.
func MarkHealthy(exe, version string) {
	b, err := os.ReadFile(markerPath(exe))
	if err != nil {
		return
	}
	if v, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n"); v == version {
		os.Remove(markerPath(exe))
	}
}

// rolledBackFrom is the version BootGuard gave up on, "" when none.
func rolledBackFrom(exe string) string {
	if exe == "" {
		return ""
	}
	b, _ := os.ReadFile(exe + ".rolledback")
	return strings.TrimSpace(string(b))
}

// skipped is the version that couldn't start, notice dismissed or not: it is
// never offered again (a newer release is).
func skipped(exe string) string {
	if v := rolledBackFrom(exe); v != "" || exe == "" {
		return v
	}
	b, _ := os.ReadFile(exe + ".skipped")
	return strings.TrimSpace(string(b))
}

// Dismiss clears the "went back to the old version" notice; the version
// stays skipped.
func (u *Updater) Dismiss() {
	if u.Exe != "" {
		os.Rename(u.Exe+".rolledback", u.Exe+".skipped") //nolint:errcheck // nothing to dismiss
	}
}
