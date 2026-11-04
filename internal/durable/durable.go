// Package durable makes a finished file survive a power cut. Without it Linux may
// keep a new file's contents in memory for up to ~30 s: after a cut the name is
// there but the file is empty.
package durable

import (
	"os"
	"path/filepath"
)

// Rename moves a file whose contents were already Sync'd, and makes the move stick.
func Rename(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	SyncDir(filepath.Dir(to))
	return nil
}

// SyncDir saves a folder's list of names. Best effort: Windows can't sync a folder
// (it needs no help here), and a failure leaves the old name, never a broken file.
func SyncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync() //nolint:errcheck // see above
		d.Close()
	}
}
