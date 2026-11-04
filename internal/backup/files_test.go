package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSafePath(t *testing.T) {
	for _, bad := range []string{"", "../x.json.gz", "/etc/x.json.gz", "a/b.json.gz", `a\b.json.gz`,
		"x.json.gz/../..", "..json.gz", "x.sql", "."} {
		if _, err := SafePath("/b", bad); err == nil {
			t.Errorf("SafePath(%q) accepted", bad)
		}
	}
	if p, err := SafePath("/b", "shop-20261008-101010.json.gz"); err != nil || p != "/b/shop-20261008-101010.json.gz" {
		t.Errorf("good name: %q %v", p, err)
	}
}

// A power cut can leave a backup with a name and no contents: it isn't a backup.
func TestListBackupsSkipsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.json.gz"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "cut.json.gz"), nil, 0o644)
	l, err := ListBackups(dir)
	if err != nil || len(l) != 1 || l[0].Name != "good.json.gz" {
		t.Fatalf("got %v %v", l, err)
	}
}

func TestListBackups(t *testing.T) {
	if l, err := ListBackups(filepath.Join(t.TempDir(), "missing")); err != nil || len(l) != 0 {
		t.Fatalf("missing dir: %v %v", l, err)
	}
	dir := t.TempDir()
	for i, n := range []string{"old.json.gz", "new.json.gz", "notes.txt"} {
		p := filepath.Join(dir, n)
		os.WriteFile(p, []byte("x"), 0o644)
		mt := time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC)
		os.Chtimes(p, mt, mt)
	}
	l, err := ListBackups(dir)
	if err != nil || len(l) != 2 || l[0].Name != "new.json.gz" {
		t.Fatalf("got %+v %v, want new then old", l, err)
	}
}
