package backup

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// CPY-3: Copy now with no new backup since the last cloud copy sends nothing
// (the bucket refuses an overwrite, and a day's quota is small).
func TestCPY3_SameBackupNeverSentTwice(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local := t.TempDir()
	writeBackups(t, local, 0, 1)
	SyncCopies(ctx, db, "", cloudOn)
	cloud := &fakeCloud{}
	now := copyBase.Add(time.Hour)
	c := newTestCopier(db, local, cloud, &now)
	for range 3 {
		if err := c.CopyNow(ctx, KindCloud); err != nil {
			t.Fatal(err)
		}
	}
	if cloud.puts != 1 {
		t.Fatalf("CPY-3: one backup sent %d times", cloud.puts)
	}
}

// CPY-2: the server's pass and Copy now at once never write the same file at
// once — every copy in the folder is whole.
func TestCPY2_CopiesAtOnce(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local, place := t.TempDir(), t.TempDir()
	names := writeBackups(t, local, 0, 2)
	big := make([]byte, 4<<20)
	for i := range big {
		big[i] = byte(i)
	}
	for _, n := range names {
		os.WriteFile(filepath.Join(local, n), big, 0o644)
	}
	SaveFolder(ctx, db, place, true)
	now := copyBase.Add(3 * time.Hour)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newTestCopier(db, local, &fakeCloud{}, &now) // each caller its own Copier, as the server and the page are
			if i%2 == 0 {
				c.RunOnce(ctx)
			} else {
				c.CopyNow(ctx, KindFolder) //nolint:errcheck
			}
		}()
	}
	wg.Wait()
	for _, n := range names {
		fi, err := os.Stat(filepath.Join(place, n))
		if err != nil || fi.Size() != int64(len(big)) {
			t.Fatalf("CPY-2: %s in the copy folder is not whole (%v)", n, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(place, "*.tmp")); len(left) != 0 {
		t.Fatalf("CPY-2: half copies left: %v", left)
	}
}

// CPY-6: Copy now on a place that isn't set up says so (nothing is tried); the
// Data page's "waiting" is how many of the newest backups a place lacks.
func TestCPY6_NotSetUpAndWaiting(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	local, place := t.TempDir(), t.TempDir()
	names := writeBackups(t, local, 0, 3)
	now := copyBase.Add(4 * time.Hour)
	c := newTestCopier(db, local, &fakeCloud{}, &now) // Keep 2
	for _, k := range []string{KindFolder, KindCloud} {
		if err := c.CopyNow(ctx, k); err == nil {
			t.Errorf("CPY-6: Copy now to an unset %s went ahead", k)
		}
	}
	os.WriteFile(filepath.Join(place, names[2]), []byte("x"), 0o644)
	if n := c.Waiting(Copy{Kind: KindFolder, Path: place}); n != 1 {
		t.Fatalf("CPY-6: waiting %d, want 1 (of the newest 2, one is there)", n)
	}
	if n := c.Waiting(Copy{Kind: KindFolder, Path: filepath.Join(place, "usb-out")}); n != 2 {
		t.Fatalf("CPY-6: waiting %d for a folder that isn't there, want 2", n)
	}
}
