package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/s3"

	"github.com/jmoiron/sqlx"
)

type fakeCloud struct {
	mu   sync.Mutex
	objs map[string][]byte
	puts int
	err  error
}

func (f *fakeCloud) Put(_ context.Context, key string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if f.objs == nil {
		f.objs = map[string][]byte{}
	}
	f.objs[key] = body
	f.puts++
	return nil
}

func (f *fakeCloud) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objs[key]
	if !ok {
		return nil, s3.ErrNotFound
	}
	return b, nil
}

func (f *fakeCloud) List(_ context.Context, prefix string) ([]s3.Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []s3.Object
	for k, v := range f.objs {
		if strings.HasPrefix(k, prefix) {
			out = append(out, s3.Object{Key: k, Size: int64(len(v))})
		}
	}
	return out, nil
}

var copyBase = time.Date(2026, 10, 20, 8, 0, 0, 0, time.Local)

// writeBackups puts n backups in dir, an hour apart from copyBase + offset hours.
func writeBackups(t *testing.T, dir string, offset, n int) []string {
	t.Helper()
	var names []string
	for i := range n {
		mt := copyBase.Add(time.Duration(offset+i) * time.Hour)
		name := FileName("Sunil Super Mart", mt, false)
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("backup "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, mt, mt)
		names = append(names, name)
	}
	return names
}

func newTestCopier(db *sqlx.DB, dir string, cloud *fakeCloud, now *time.Time) *Copier {
	return &Copier{DB: db, Dir: dir, Keep: 2, Cloud: cloudOn, S3: cloud, Now: func() time.Time { return *now }}
}

func TestCopyFolderCatchUpAndPrune(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local, place := t.TempDir(), t.TempDir()
	names := writeBackups(t, local, 0, 4)
	os.WriteFile(filepath.Join(place, names[1]), []byte("old"), 0o644)
	os.Chtimes(filepath.Join(place, names[1]), copyBase.Add(time.Hour), copyBase.Add(time.Hour))
	os.WriteFile(filepath.Join(place, "stray.json.gz.tmp"), []byte("half"), 0o644)
	SaveFolder(ctx, db, place, true)
	now := copyBase.Add(5 * time.Hour)
	newTestCopier(db, local, &fakeCloud{}, &now).RunOnce(ctx)

	got, _ := ListBackups(place)
	if len(got) != 2 || got[0].Name != names[3] || got[1].Name != names[2] {
		t.Fatalf("place holds %+v, want the newest two", got)
	}
	if !got[0].ModifiedAt.Equal(copyBase.Add(3 * time.Hour)) {
		t.Fatalf("copy lost its time: %v", got[0].ModifiedAt)
	}
	if _, err := os.Stat(filepath.Join(place, "stray.json.gz.tmp")); err != nil {
		t.Fatal("a stray .tmp was touched by the prune")
	}
	if left, _ := ListBackups(local); len(left) != 4 {
		t.Fatalf("local backups changed: %d", len(left))
	}
	f := copyRow(t, db, KindFolder)
	if f.LastError != "" || f.LastCopiedAt == nil || f.LastFile != names[3] {
		t.Fatalf("status = %+v", f)
	}
}

func TestCopyFolderMissingIsSkipped(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local := t.TempDir()
	writeBackups(t, local, 0, 1)
	gone := filepath.Join(t.TempDir(), "usb")
	SaveFolder(ctx, db, gone, true)
	now := copyBase
	newTestCopier(db, local, &fakeCloud{}, &now).RunOnce(ctx)
	if _, err := os.Stat(gone); err == nil {
		t.Fatal("the pass created a missing copy folder")
	}
	if f := copyRow(t, db, KindFolder); !strings.Contains(f.LastError, "not there") || f.LastTriedAt == nil {
		t.Fatalf("status = %+v", f)
	}
}

func TestCopyDisabledFolderUntouched(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local, place := t.TempDir(), t.TempDir()
	writeBackups(t, local, 0, 1)
	SaveFolder(ctx, db, place, false)
	now := copyBase
	newTestCopier(db, local, &fakeCloud{}, &now).RunOnce(ctx)
	if got, _ := ListBackups(place); len(got) != 0 {
		t.Fatal("a switched-off folder got a copy")
	}
}

func TestCopyCloudDaily(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local := t.TempDir()
	writeBackups(t, local, 0, 2)
	SyncCopies(ctx, db, "", cloudOn)
	cloud := &fakeCloud{}
	now := copyBase.Add(2 * time.Hour)
	c := newTestCopier(db, local, cloud, &now)

	c.RunOnce(ctx)
	if cloud.puts != 1 {
		t.Fatalf("puts = %d, want 1", cloud.puts)
	}
	want := "shop/" + copyBase.Add(time.Hour).Format("20060102-150405") + SealedSuffix
	body, ok := cloud.objs[want]
	if !ok {
		t.Fatalf("cloud holds %v, want %s (no shop name)", cloud.objs, want)
	}
	key, _ := ParseKey(testKeyHex)
	if plain, err := Open(key, body); err != nil || !strings.HasPrefix(string(plain), "backup ") {
		t.Fatalf("cloud copy doesn't open: %q %v", plain, err)
	}

	writeBackups(t, local, 2, 1)
	now = now.Add(time.Hour)
	c.RunOnce(ctx)
	if cloud.puts != 1 {
		t.Fatal("uploaded again within 20 h")
	}
	now = now.Add(21 * time.Hour)
	c.RunOnce(ctx)
	if cloud.puts != 2 {
		t.Fatalf("puts after 21 h = %d, want 2", cloud.puts)
	}

	writeBackups(t, local, 3, 1)
	db.MustExec(`UPDATE backup_copies SET last_copied_at = $1 WHERE kind = 'cloud'`, now.Add(2*time.Hour))
	c.RunOnce(ctx)
	if cloud.puts != 3 {
		t.Fatal("a last copy in the future (clock wrong) blocked the upload")
	}
}

func TestCopyCloudFailureStored(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local := t.TempDir()
	writeBackups(t, local, 0, 1)
	SyncCopies(ctx, db, "", cloudOn)
	now := copyBase
	newTestCopier(db, local, &fakeCloud{err: errors.New("no internet")}, &now).RunOnce(ctx)
	if c := copyRow(t, db, KindCloud); !strings.Contains(c.LastError, "no internet") || c.LastCopiedAt != nil {
		t.Fatalf("status = %+v", c)
	}
}

func TestTestFolder(t *testing.T) {
	parent := t.TempDir()
	c := &Copier{}
	if err := c.TestFolder(filepath.Join(parent, "Karots copies")); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(filepath.Join(parent, "Karots copies")); len(entries) != 0 {
		t.Fatal("Test left a file behind")
	}
	if err := c.TestFolder(filepath.Join(parent, "no", "such")); err == nil {
		t.Fatal("Test made a folder whose parent is missing")
	}
}
