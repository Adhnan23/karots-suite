package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Fetching back: only a listed bare name, the backup's own time kept, the cloud
// copy opened with the key — and a fetched old backup never counts as newest.
func TestFetchFromCopies(t *testing.T) {
	ctx := context.Background()
	local, place := t.TempDir(), t.TempDir()
	names := writeBackups(t, place, 0, 2)
	cloud := &fakeCloud{}
	now := copyBase.Add(10 * time.Hour)
	c := newTestCopier(nil, local, cloud, &now)
	folder := Copy{Kind: KindFolder, Path: place}

	for _, bad := range []string{"../" + names[0], "other.json.gz", ""} {
		if _, err := c.Fetch(ctx, folder, bad); !errors.Is(err, ErrNotListed) {
			t.Fatalf("fetch %q: %v", bad, err)
		}
	}
	if c.Waiting(folder) != 0 {
		t.Fatal("waiting with no local backups")
	}
	got, err := c.Fetch(ctx, folder, names[0])
	if err != nil || got != names[0] {
		t.Fatalf("folder fetch: %q %v", got, err)
	}
	if st, _ := os.Stat(filepath.Join(local, got)); !st.ModTime().Equal(copyBase) {
		t.Fatalf("fetched copy has time %v", st.ModTime())
	}
	if c.Waiting(folder) != 0 || c.Waiting(Copy{Kind: KindFolder, Path: filepath.Join(place, "gone")}) != 1 {
		t.Fatal("waiting count")
	}

	key, _ := ParseKey(testKeyHex)
	sealed, _ := Seal(key, []byte("cloud backup"))
	cloud.objs = map[string][]byte{"shop/" + CloudName(copyBase.Add(time.Hour)): sealed, "shop/notes.txt": []byte("x")}
	cl := Copy{Kind: KindCloud}
	list, err := c.Backups(ctx, cl)
	if err != nil || len(list) != 1 || !list[0].ModifiedAt.Equal(copyBase.Add(time.Hour)) {
		t.Fatalf("cloud list: %+v %v", list, err)
	}
	got, err = c.Fetch(ctx, cl, list[0].Name)
	if err != nil || got != "cloud-20261020-090000.json.gz" {
		t.Fatalf("cloud fetch: %q %v", got, err)
	}
	if b, _ := os.ReadFile(filepath.Join(local, got)); string(b) != "cloud backup" {
		t.Fatalf("cloud fetch body %q", b)
	}
	if c.Waiting(cl) != 1 {
		t.Fatal("never-copied cloud should be due")
	}
}
