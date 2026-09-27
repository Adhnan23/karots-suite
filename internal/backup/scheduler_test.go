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
