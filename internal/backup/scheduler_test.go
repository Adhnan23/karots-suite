package backup

import (
	"os"
	"path/filepath"
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
