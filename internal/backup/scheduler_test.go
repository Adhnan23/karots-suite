package backup

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRotateKeepsNewest(t *testing.T) {
	dir := t.TempDir()

	// 30 backup files with ascending modification times (oldest first).
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var names []string
	for i := range 30 {
		mt := base.Add(time.Duration(i) * time.Hour)
		name := FileName("Acme", mt, false)
		names = append(names, name)
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	// An unrelated file must be ignored by rotation.
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	pruned, err := Rotate(dir, 28)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("pruned = %d, want 2", pruned)
	}

	// The two oldest are gone; the newest 28 and the unrelated file remain.
	if _, err := os.Stat(filepath.Join(dir, names[0])); !os.IsNotExist(err) {
		t.Errorf("oldest backup %s should have been pruned", names[0])
	}
	if _, err := os.Stat(filepath.Join(dir, names[1])); !os.IsNotExist(err) {
		t.Errorf("second-oldest backup %s should have been pruned", names[1])
	}
	if _, err := os.Stat(filepath.Join(dir, names[2])); err != nil {
		t.Errorf("backup %s should have been kept: %v", names[2], err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file should not be touched: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "*"+fileSuffix))
	if len(matches) != 28 {
		t.Fatalf("remaining backups = %d, want 28", len(matches))
	}
}

// Empty files left by power cuts go, and never push a real backup out.
func TestRotateDropsEmptyWithoutCountingThem(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json.gz")
	os.WriteFile(good, []byte("x"), 0o644)
	older := time.Now().Add(-time.Hour) // the empty ones are newer: by age alone the good one would go
	os.Chtimes(good, older, older)
	for _, n := range []string{"a.json.gz", "b.json.gz", "c.json.gz"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	if _, err := Rotate(dir, 1); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.json.gz"))
	if len(left) != 1 || left[0] != good {
		t.Fatalf("left %v", left)
	}
}

func TestRotateNoopWhenUnderLimit(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		name := FileName("Acme", time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC), false)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pruned, err := Rotate(dir, 28)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("pruned = %d, want 0 when under the limit", pruned)
	}
}

// BAK-4: safety copies (before a restore, an update, a move) and backups
// fetched from Karots cloud are never counted by rotation: each is kept 30
// days, then deleted.
func TestBAK4_SafetyCopiesKept30Days(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	put := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, now.Add(-age), now.Add(-age))
		return p
	}
	day := 24 * time.Hour
	oldAuto := put(FileName("Acme", now.Add(-40*day), false), 40*day)
	var autos []string
	for i := range 3 {
		autos = append(autos, put(FileName("Acme", now.Add(-time.Duration(i)*time.Hour), false), time.Duration(i)*time.Hour))
	}
	young := []string{
		put("acme-20261101-100000-before-restore.json.gz", 20*day),
		put("acme-20261101-110000-before-update-2026.11.17.2028.json.gz", 29*day),
		put("acme-20261101-120000-before-move.json.gz", 2*day),
		put("cloud-20261101-130000.json.gz", 10*day),
	}
	old := []string{
		put("acme-20261001-100000-before-restore.json.gz", 31*day),
		put("cloud-20261001-130000.json.gz", 45*day),
	}
	if _, err := Rotate(dir, 3); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(autos, young...) {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("BAK-4: %s should be kept", filepath.Base(p))
		}
	}
	for _, p := range append(old, oldAuto) {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("BAK-4: %s should be gone", filepath.Base(p))
		}
	}
}

// BAK-12: the scheduler backs up at start and every interval, and a failed
// backup never stops it — the next one tries again.
func TestBAK12_SchedulerKeepsGoing(t *testing.T) {
	db := copiesDB(t)
	parent := t.TempDir()
	dir := filepath.Join(parent, "backups")
	if err := os.WriteFile(dir, []byte("in the way"), 0o644); err != nil { // the folder can't be made: backups fail
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var after atomic.Int32
	go func() {
		RunScheduler(ctx, db, dir, 50*time.Millisecond, 5, func() { after.Add(1) })
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	if after.Load() != 0 {
		t.Fatal("copies kicked for a backup that failed")
	}
	os.Remove(dir) // the folder can be made now
	deadline := time.Now().Add(5 * time.Second)
	for after.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	if after.Load() < 2 {
		t.Fatalf("BAK-12: %d backups after the folder came back", after.Load())
	}
	if list, _ := ListBackups(dir); len(list) == 0 || len(list) > 5 {
		t.Fatalf("BAK-12: %d backups in the folder (keep 5)", len(list))
	}
}

// BAK-4: BACKUP_KEEP=0 never prunes an automatic backup.
func TestRotateKeepZeroKeepsAll(t *testing.T) {
	dir := t.TempDir()
	for i := range 3 {
		os.WriteFile(filepath.Join(dir, FileName("Acme", time.Now().Add(-time.Duration(i)*time.Hour), false)), []byte("x"), 0o644)
	}
	if _, err := Rotate(dir, 0); err != nil {
		t.Fatal(err)
	}
	if list, _ := ListBackups(dir); len(list) != 3 {
		t.Fatalf("BAK-4: keep 0 left %d of 3", len(list))
	}
}
