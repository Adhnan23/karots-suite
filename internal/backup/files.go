package backup

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BeforeRestoreTag marks the safety copy written just before a restore.
const BeforeRestoreTag = "before-restore"

// BackupFile is one backup on disk, for the Data page's list.
type BackupFile struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// ListBackups returns the *.json.gz files in dir, newest first. A missing dir
// is an empty list, not an error.
func ListBackups(dir string) ([]BackupFile, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []BackupFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []BackupFile{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), fileSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupFile{Name: e.Name(), Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModifiedAt.After(out[j].ModifiedAt) })
	return out, nil
}

// ErrBadName is returned for a client-supplied backup name that is not a plain
// backup file name in the folder.
var ErrBadName = errors.New("not a backup file name")

// SafePath turns a client-supplied file name into a path inside dir, refusing
// anything that is not a bare *.json.gz name (path traversal guard).
func SafePath(dir, name string) (string, error) {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") ||
		strings.ContainsAny(name, `/\`) || !strings.HasSuffix(name, fileSuffix) {
		return "", ErrBadName
	}
	return filepath.Join(dir, name), nil
}
