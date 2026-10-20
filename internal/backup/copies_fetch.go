package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNotListed: a name the copy place doesn't list (the client never names a path).
var ErrNotListed = errors.New("that backup is not in this copy")

// Waiting is how far a place is behind: of the newest Keep local backups, the
// ones the copy folder lacks (all while it isn't there), or 1 when the cloud
// is due a copy.
func (c *Copier) Waiting(r Copy) int {
	local, err := c.newest()
	if err != nil || len(local) == 0 {
		return 0
	}
	if r.Kind == KindCloud {
		if cloudDue(r, c.Now()) {
			return 1
		}
		return 0
	}
	if st, err := os.Stat(r.Path); r.Path == "" || err != nil || !st.IsDir() {
		return len(local)
	}
	there, err := ListBackups(r.Path)
	if err != nil {
		return 0
	}
	have := map[string]bool{}
	for _, f := range there {
		have[f.Name] = true
	}
	n := 0
	for _, f := range local {
		if !have[f.Name] {
			n++
		}
	}
	return n
}

// Backups lists the backups in a copy place, newest first (cloud names
// without the shop's folder; their time is the one in the name).
func (c *Copier) Backups(ctx context.Context, r Copy) ([]BackupFile, error) {
	if r.Kind == KindFolder {
		if st, err := os.Stat(r.Path); r.Path == "" || err != nil || !st.IsDir() {
			return nil, errors.New("not there right now: " + r.Path)
		}
		return ListBackups(r.Path)
	}
	objs, err := c.S3.List(ctx, c.Cloud.Folder+"/")
	if err != nil {
		return nil, err
	}
	out := []BackupFile{}
	for _, o := range objs {
		name := strings.TrimPrefix(o.Key, c.Cloud.Folder+"/")
		if !hasCloudSuffix(name) || strings.Contains(name, "/") {
			continue
		}
		mod, err := time.ParseInLocation("20060102-150405", strings.TrimSuffix(name, SealedSuffix), time.Local)
		if err != nil {
			mod = o.Modified
		}
		out = append(out, BackupFile{Name: name, Size: o.Size, ModifiedAt: mod})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModifiedAt.After(out[j].ModifiedAt) })
	return out, nil
}

// Fetch brings one listed backup from a copy place into the backups folder,
// ready for the normal restore, and returns its local name. It keeps the
// backup's own time, or the cloud pass would take it for the newest backup.
func (c *Copier) Fetch(ctx context.Context, r Copy, name string) (string, error) {
	list, err := c.Backups(ctx, r)
	if err != nil {
		return "", err
	}
	var f *BackupFile
	for i := range list {
		if list[i].Name == name {
			f = &list[i]
		}
	}
	if f == nil {
		return "", ErrNotListed
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return "", err
	}
	if r.Kind == KindFolder {
		return name, copyInto(filepath.Join(r.Path, name), filepath.Join(c.Dir, name), f.ModifiedAt)
	}
	key, err := ParseKey(c.Cloud.Crypt)
	if err != nil {
		return "", err
	}
	sealed, err := c.S3.Get(ctx, c.Cloud.Folder+"/"+name)
	if err != nil {
		return "", err
	}
	plain, err := Open(key, sealed)
	if err != nil {
		return "", err
	}
	local := "cloud-" + strings.TrimSuffix(name, ".enc")
	p := filepath.Join(c.Dir, local)
	if err := os.WriteFile(p+".tmp", plain, 0o644); err != nil {
		return "", err
	}
	os.Chtimes(p+".tmp", f.ModifiedAt, f.ModifiedAt) //nolint:errcheck // the time only orders the backups
	return local, os.Rename(p+".tmp", p)
}
