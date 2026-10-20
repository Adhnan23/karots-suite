package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// fileSuffix is the extension every backup (scheduled or manual) carries; the
// YYYYMMDD-HHMMSS timestamp in the name makes them sort chronologically. See
// FileName in name.go for the full naming scheme.
const fileSuffix = ".json.gz"

// WriteSnapshot dumps all data to dir/<shop>-<timestamp>[-tag].json.gz, returning
// the final path (tag "" for the scheduled ones, "before-restore" for the safety copy). It writes to a temporary file and renames it into place so a crash
// mid-dump never leaves a half-written ".gz" that looks restorable.
func WriteSnapshot(ctx context.Context, db *sqlx.DB, dir string, now time.Time, tag string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	name := FileName(shopName(ctx, db), now, false)
	if tag != "" {
		name = strings.TrimSuffix(name, fileSuffix) + "-" + tag + fileSuffix
	}
	final := filepath.Join(dir, name)
	tmp := final + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	if err := Dump(ctx, db, now.Format(time.RFC3339), f, DumpOpts{}); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("finalize backup file: %w", err)
	}
	return final, nil
}

// Rotate deletes the oldest scheduled backups in dir, keeping the newest `keep`.
// Files are matched by the *.json.gz name and ordered by modification time
// (names now vary by shop slug, so name-sort alone no longer tracks age).
// keep <= 0 disables pruning.
func Rotate(dir string, keep int) (pruned int, err error) {
	if keep <= 0 {
		return 0, nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*"+fileSuffix))
	if err != nil {
		return 0, err
	}
	if len(matches) <= keep {
		return 0, nil
	}
	// Oldest first by modification time (filenames vary by shop slug, so a name
	// sort no longer tracks age across a shop rename).
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		return fi != nil && fj != nil && fi.ModTime().Before(fj.ModTime())
	})
	for _, old := range matches[:len(matches)-keep] {
		if rmErr := os.Remove(old); rmErr != nil {
			err = rmErr // report the last failure but keep going
			continue
		}
		pruned++
	}
	return pruned, err
}

// RunScheduler writes a backup immediately, then every `interval`, pruning to the
// newest `keep` files each time. All outcomes are logged; errors never stop the
// loop (a transient backup failure must not take down the POS). It returns when
// ctx is cancelled. Intended to be launched with `go RunScheduler(...)`. after
// (may be nil) runs after each backup written — it starts the copies.
func RunScheduler(ctx context.Context, db *sqlx.DB, dir string, interval time.Duration, keep int, after func()) {
	runOnce := func() {
		path, err := WriteSnapshot(ctx, db, dir, time.Now(), "")
		if err != nil {
			log.Printf("auto-backup: snapshot failed: %v", err)
			return
		}
		if after != nil {
			after()
		}
		pruned, err := Rotate(dir, keep)
		if err != nil {
			log.Printf("auto-backup: wrote %s but rotation had an error: %v", path, err)
			return
		}
		log.Printf("auto-backup: wrote %s (pruned %d old, keep %d)", path, pruned, keep)
	}

	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("auto-backup: stopping")
			return
		case <-ticker.C:
			runOnce()
		}
	}
}
