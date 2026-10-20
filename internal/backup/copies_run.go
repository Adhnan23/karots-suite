package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/config"
	"karots-suite/internal/s3"

	"github.com/jmoiron/sqlx"
)

// CloudStore is the Karots cloud bucket (s3.Client; a fake in tests).
type CloudStore interface {
	Put(ctx context.Context, key string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) ([]s3.Object, error)
}

// cloudEvery: the cloud gets the newest backup once its last copy is this old —
// about one a day (the vendor's bucket is on R2's free plan).
const cloudEvery = 20 * time.Hour

// copyEvery is how often a pass runs besides right after each backup: a USB
// plugged back in catches up within half an hour.
const copyEvery = 30 * time.Minute

// Copier copies the backups folder (Dir) to the copy folder and Karots cloud.
// Nothing it does reaches the till: errors go to the log and backup_copies.
type Copier struct {
	DB    *sqlx.DB
	Dir   string // the local backups folder
	Keep  int    // the copy folder keeps this many (BACKUP_KEEP)
	Cloud config.CloudBackup
	S3    CloudStore
	Now   func() time.Time
}

// NewCopier is the server's copier.
func NewCopier(db *sqlx.DB, dir string, keep int, cloud config.CloudBackup) *Copier {
	return &Copier{DB: db, Dir: dir, Keep: keep, Cloud: cloud, Now: time.Now,
		S3: s3.Client{URL: cloud.URL, KeyID: cloud.KeyID, Secret: cloud.Secret}}
}

// errNotDue: the cloud's last copy is recent — nothing tried, status untouched.
var errNotDue = errors.New("not due")

// Run copies on every kick (a backup was just written) and every 30 minutes.
func (c *Copier) Run(ctx context.Context, kick <-chan struct{}) {
	t := time.NewTicker(copyEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-kick:
		case <-t.C:
		}
		c.RunOnce(ctx)
	}
}

// RunOnce is one pass over the switched-on places.
func (c *Copier) RunOnce(ctx context.Context) {
	rows, err := Copies(ctx, c.DB)
	if err != nil {
		log.Printf("backup copies: %v", err)
		return
	}
	for _, r := range rows {
		if r.Enabled {
			c.run(ctx, r, false) //nolint:errcheck // logged and stored
		}
	}
}

// CopyNow copies to one place now (the Data page's Copy now), the cloud even
// if its last copy is recent, and returns how it went.
func (c *Copier) CopyNow(ctx context.Context, kind string) error {
	r, err := CopyOf(ctx, c.DB, kind)
	if err != nil {
		return err
	}
	if r == nil || (kind == KindFolder && r.Path == "") {
		return fmt.Errorf("no %s copy set up", kind)
	}
	return c.run(ctx, *r, true)
}

// passMu keeps one copy at a time: the server's pass and a Data page Copy now
// (each with its own Copier) would otherwise write the same .tmp.
var passMu sync.Mutex

func (c *Copier) run(ctx context.Context, r Copy, force bool) error {
	passMu.Lock()
	defer passMu.Unlock()
	var file string
	var err error
	switch r.Kind {
	case KindFolder:
		file, err = c.copyFolder(r.Path)
	case KindCloud:
		file, err = c.copyCloud(ctx, r, force)
		if errors.Is(err, errNotDue) {
			return nil
		}
	default:
		return fmt.Errorf("unknown copy %q", r.Kind)
	}
	if err != nil {
		log.Printf("backup copies: %s: %v", r.Kind, err)
	}
	copyTried(ctx, c.DB, r.Kind, file, c.Now(), err)
	return err
}

// copyFolder copies every local backup the folder lacks (oldest first), then
// keeps the newest Keep there. A folder that isn't there (USB not plugged in)
// is skipped, never created. It returns the newest file copied ("" = none new).
func (c *Copier) copyFolder(path string) (string, error) {
	if path == "" {
		return "", errors.New("no copy folder set")
	}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return "", fmt.Errorf("not there right now: %s", path)
	}
	local, err := ListBackups(c.Dir)
	if err != nil {
		return "", err
	}
	there, err := ListBackups(path)
	if err != nil {
		return "", err
	}
	have := map[string]bool{}
	for _, f := range there {
		have[f.Name] = true
	}
	newest := ""
	for i := len(local) - 1; i >= 0; i-- { // oldest first
		f := local[i]
		if have[f.Name] {
			continue
		}
		if err := copyInto(filepath.Join(c.Dir, f.Name), filepath.Join(path, f.Name), f.ModifiedAt); err != nil {
			return newest, err
		}
		newest = f.Name
	}
	if _, err := Rotate(path, c.Keep); err != nil {
		return newest, err
	}
	return newest, nil
}

// copyInto writes dst through a .tmp (a pulled USB never leaves a file that
// looks restorable) and keeps the backup's own time.
func copyInto(src, dst string, mod time.Time) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	os.Chtimes(tmp, mod, mod) //nolint:errcheck // the time only orders the copies
	return os.Rename(tmp, dst)
}

// CloudName is a backup's name in Karots cloud: its time only, no shop name.
func CloudName(mod time.Time) string { return mod.Format("20060102-150405") + SealedSuffix }

// copyCloud seals the newest local backup into the shop's cloud folder when
// the last cloud copy is over 20 h old (or in the future: a wrong clock).
// The shop never deletes there: the bucket's own rules keep 30 days.
func (c *Copier) copyCloud(ctx context.Context, r Copy, force bool) (string, error) {
	now := c.Now()
	if !force && !cloudDue(r, now) {
		return "", errNotDue
	}
	local, err := ListBackups(c.Dir)
	if err != nil {
		return "", err
	}
	if len(local) == 0 {
		return "", errors.New("no backup to copy yet")
	}
	name := CloudName(local[0].ModifiedAt)
	if name == r.LastFile {
		return "", nil // already there (the bucket refuses an overwrite)
	}
	key, err := ParseKey(c.Cloud.Crypt)
	if err != nil {
		return "", err
	}
	plain, err := os.ReadFile(filepath.Join(c.Dir, local[0].Name))
	if err != nil {
		return "", err
	}
	sealed, err := Seal(key, plain)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := c.S3.Put(ctx, c.Cloud.Folder+"/"+name, sealed); err != nil {
		return "", err
	}
	return name, nil
}

// TestFolder checks the copy folder can take a file: it is created when its
// parent exists (a USB stick's new "Karots copies" folder), then a small file
// is written and removed.
func (c *Copier) TestFolder(path string) error {
	if err := os.Mkdir(path, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("can't use %s: %w", path, err)
	}
	p := filepath.Join(path, "karots-test.tmp")
	if err := os.WriteFile(p, []byte("Karots copy test"), 0o644); err != nil {
		return fmt.Errorf("can't write in %s: %w", path, err)
	}
	return os.Remove(p)
}

// cloudDue: never copied, last copy over 20 h old, or in the future (a wrong clock).
func cloudDue(r Copy, now time.Time) bool {
	return r.LastCopiedAt == nil || r.LastCopiedAt.After(now) || now.Sub(*r.LastCopiedAt) > cloudEvery
}

// hasCloudSuffix is a name in the cloud list (not the bucket's other files).
func hasCloudSuffix(key string) bool { return strings.HasSuffix(key, SealedSuffix) }
